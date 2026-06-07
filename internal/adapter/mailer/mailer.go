package mailer

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/wneessen/go-mail"

	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/infrastructure/metrics"
)

const (
	sendTimeout      = time.Minute
	jobBufferSize    = 64
	kindConfirmation = "confirmation"
	kindNotification = "notification"
)

var confirmationTemplate = template.Must(template.New("confirmation").Parse(`<!DOCTYPE html>
<html>
<body>
    <h2>Confirm your subscription</h2>
    <p>You requested to subscribe to new releases of <strong>{{.Repo}}</strong>.</p>
    <p>Click <a href="{{.ConfirmURL}}">here</a> to confirm your subscription.</p>
    <p>If you did not request this, please ignore this email.</p>
</body>
</html>`))

var releaseTemplate = template.Must(template.New("release").Parse(`<!DOCTYPE html>
<html>
<body>
    <h2>New release for {{.Repo}}</h2>
    <p>A new release <strong>{{.Tag}}</strong> is available.</p>
    <p><a href="{{.ReleaseURL}}">View</a> release on GitHub</p>
    <br>
    <p><small><a href="{{.UnsubscribeURL}}">Unsubscribe</a></small></p>
</body>
</html>`))

type sender interface {
	sendBatch(ctx context.Context, messages []*mail.Msg) []error
}

type mailJob struct {
	ctx      context.Context
	kind     string
	messages []*mail.Msg
	result   chan<- []error
}

type Mailer struct {
	sender    sender
	fromEmail string
	jobs      chan mailJob
	wg        sync.WaitGroup
	log       *slog.Logger
}

func NewMailer(host string, port int, user, password, fromEmail string, log *slog.Logger) (*Mailer, error) {
	client, err := mail.NewClient(host,
		mail.WithPort(port),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(user),
		mail.WithPassword(password),
	)
	if err != nil {
		return nil, fmt.Errorf("create mail client: %w", err)
	}

	return newMailer(&smtpSender{client: client, log: log}, fromEmail, log), nil
}

func newMailer(s sender, fromEmail string, log *slog.Logger) *Mailer {
	m := &Mailer{
		sender:    s,
		fromEmail: fromEmail,
		jobs:      make(chan mailJob, jobBufferSize),
		log:       log.With("component", "mailer"),
	}

	m.wg.Add(1)
	go m.dispatch()

	return m
}

func (m *Mailer) dispatch() {
	defer m.wg.Done()

	for job := range m.jobs {
		m.deliver(job)
	}
}

func (m *Mailer) deliver(job mailJob) {
	ctx, cancel := context.WithTimeout(job.ctx, sendTimeout)
	defer cancel()

	start := time.Now()
	errs := m.sender.sendBatch(ctx, job.messages)
	metrics.EmailSendDuration.WithLabelValues(job.kind).Observe(time.Since(start).Seconds())

	for i, err := range errs {
		metrics.EmailSendsTotal.WithLabelValues(job.kind, metrics.ResultLabel(err)).Inc()
		if err != nil {
			m.log.ErrorContext(
				ctx,
				"failed to send email",
				"kind",
				job.kind,
				"to",
				recipient(job.messages[i]),
				"error",
				err,
			)
		}
	}

	if job.result != nil {
		job.result <- errs
	}
}

func (m *Mailer) SendConfirmation(ctx context.Context, to, repo, confirmURL string) {
	body, err := renderTemplate(confirmationTemplate, map[string]string{
		"Repo":       repo,
		"ConfirmURL": confirmURL,
	})
	if err != nil {
		m.log.ErrorContext(ctx, "failed to render confirmation email", "to", to, "repo", repo, "error", err)
		return
	}

	msg, err := m.buildMessage(to, fmt.Sprintf("Confirm your subscription to %s", repo), body)
	if err != nil {
		m.log.ErrorContext(ctx, "failed to build confirmation email", "to", to, "repo", repo, "error", err)
		return
	}

	job := mailJob{ctx: context.WithoutCancel(ctx), kind: kindConfirmation, messages: []*mail.Msg{msg}}

	select {
	case m.jobs <- job:
	case <-ctx.Done():
		m.log.WarnContext(ctx, "context done before confirmation could be queued", "to", to, "repo", repo)
	}
}

func (m *Mailer) SendReleaseNotifications(
	ctx context.Context,
	notifications []entity.ReleaseNotification,
) entity.BatchResult {
	var result entity.BatchResult

	messages := make([]*mail.Msg, 0, len(notifications))
	recipients := make([]string, 0, len(notifications))

	for i := range notifications {
		n := &notifications[i]

		msg, err := m.buildReleaseMessage(n)
		if err != nil {
			m.log.ErrorContext(ctx, "failed to build release email", "to", n.To, "error", err)
			result.Failed = append(result.Failed, n.To)
			continue
		}

		messages = append(messages, msg)
		recipients = append(recipients, n.To)
	}

	if len(messages) == 0 {
		return result
	}

	resultCh := make(chan []error, 1)
	m.jobs <- mailJob{ctx: ctx, kind: kindNotification, messages: messages, result: resultCh}
	errs := <-resultCh

	for i, err := range errs {
		if err != nil {
			result.Failed = append(result.Failed, recipients[i])
		} else {
			result.Sent++
		}
	}

	return result
}

func (m *Mailer) Shutdown(ctx context.Context) {
	close(m.jobs)

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.log.Info("mailer stopped")
	case <-ctx.Done():
		m.log.WarnContext(ctx, "mailer shutdown timed out, queued emails may be dropped")
	}
}

func (m *Mailer) buildReleaseMessage(n *entity.ReleaseNotification) (*mail.Msg, error) {
	body, err := renderTemplate(releaseTemplate, map[string]string{
		"Repo":           n.Repo,
		"Tag":            n.Tag,
		"ReleaseURL":     n.ReleaseURL,
		"UnsubscribeURL": n.UnsubscribeURL,
	})
	if err != nil {
		return nil, fmt.Errorf("render release email: %w", err)
	}

	return m.buildMessage(n.To, fmt.Sprintf("New release %s for %s", n.Tag, n.Repo), body)
}

func (m *Mailer) buildMessage(to, subject, body string) (*mail.Msg, error) {
	msg := mail.NewMsg()
	if err := msg.From(m.fromEmail); err != nil {
		return nil, fmt.Errorf("set from: %w", err)
	}

	if err := msg.To(to); err != nil {
		return nil, fmt.Errorf("set to: %w", err)
	}

	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextHTML, body)

	return msg, nil
}

func recipient(msg *mail.Msg) string {
	to := msg.GetTo()
	if len(to) == 0 {
		return ""
	}

	return to[0].Address
}

func renderTemplate(tmpl *template.Template, data map[string]string) (string, error) {
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}
