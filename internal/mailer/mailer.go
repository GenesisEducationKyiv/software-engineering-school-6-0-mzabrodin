package mailer

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"strings"
	"time"

	"github-release-notifier/internal/domain"
	"github-release-notifier/internal/metrics"

	"github.com/wneessen/go-mail"
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

type Mailer struct {
	client    *mail.Client
	fromEmail string
	log       *slog.Logger
}

func NewMailer(host string, port int, user, password, fromEmail string, log *slog.Logger) (*Mailer, error) {
	c, err := mail.NewClient(host,
		mail.WithPort(port),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(user),
		mail.WithPassword(password),
	)
	if err != nil {
		return nil, fmt.Errorf("create mail client: %w", err)
	}

	return &Mailer{client: c, fromEmail: fromEmail, log: log.With("component", "mailer")}, nil
}

func (m *Mailer) SendConfirmation(ctx context.Context, to, repo, confirmURL string) (err error) {
	start := time.Now()
	defer func() {
		metrics.EmailSendsTotal.WithLabelValues("confirmation", metrics.ResultLabel(err)).Inc()
		metrics.EmailSendDuration.WithLabelValues("confirmation").Observe(time.Since(start).Seconds())
	}()

	body, err := renderTemplate(confirmationTemplate, map[string]string{
		"Repo":       repo,
		"ConfirmURL": confirmURL,
	})
	if err != nil {
		return fmt.Errorf("render confirmation email: %w", err)
	}

	msg := mail.NewMsg()
	if err := msg.From(m.fromEmail); err != nil {
		return fmt.Errorf("set from: %w", err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("set to: %w", err)
	}
	msg.Subject(fmt.Sprintf("Confirm your subscription to %s", repo))
	msg.SetBodyString(mail.TypeTextHTML, body)

	if err := m.client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

func (m *Mailer) SendReleaseNotifications(ctx context.Context, notifications []domain.ReleaseNotification) (err error) {
	start := time.Now()
	defer func() {
		metrics.EmailSendsTotal.WithLabelValues("notification", metrics.ResultLabel(err)).Inc()
		metrics.EmailSendDuration.WithLabelValues("notification").Observe(time.Since(start).Seconds())
	}()

	if len(notifications) == 0 {
		return nil
	}

	if err := m.client.DialWithContext(ctx); err != nil {
		return fmt.Errorf("dial SMTP: %w", err)
	}
	defer func() {
		if err := m.client.Close(); err != nil {
			m.log.ErrorContext(ctx, "failed to close SMTP connection", "error", err)
		}
	}()

	var errs []error

	for _, n := range notifications {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}

		if err := m.sendNotification(&n); err != nil {
			m.log.ErrorContext(
				ctx,
				"failed to send release notification",
				"to",
				n.To,
				"repo",
				n.Repo,
				"tag",
				n.Tag,
				"error",
				err,
			)
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (m *Mailer) sendNotification(n *domain.ReleaseNotification) error {
	body, err := renderTemplate(releaseTemplate, map[string]string{
		"Repo":           n.Repo,
		"Tag":            n.Tag,
		"ReleaseURL":     n.ReleaseURL,
		"UnsubscribeURL": n.UnsubscribeURL,
	})
	if err != nil {
		return fmt.Errorf("render email for %s: %w", n.To, err)
	}

	msg := mail.NewMsg()
	if err := msg.From(m.fromEmail); err != nil {
		return fmt.Errorf("set from for %s: %w", n.To, err)
	}
	if err := msg.To(n.To); err != nil {
		return fmt.Errorf("set to for %s: %w", n.To, err)
	}
	msg.Subject(fmt.Sprintf("New release %s for %s", n.Tag, n.Repo))
	msg.SetBodyString(mail.TypeTextHTML, body)

	if err := m.client.Send(msg); err != nil {
		return fmt.Errorf("send email to %s: %w", n.To, err)
	}

	return nil
}

func renderTemplate(tmpl *template.Template, data map[string]string) (string, error) {
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}
