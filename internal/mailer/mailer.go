package mailer

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"strings"

	"github-release-notifier/internal/domain"

	"gopkg.in/gomail.v2"
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
	dialer    *gomail.Dialer
	fromEmail string
}

func NewMailer(host string, port int, user, password, fromEmail string) *Mailer {
	return &Mailer{
		dialer:    gomail.NewDialer(host, port, user, password),
		fromEmail: fromEmail,
	}
}

func (m *Mailer) SendConfirmation(to, repo, confirmURL string) error {
	body, err := renderTemplate(confirmationTemplate, map[string]string{
		"Repo":       repo,
		"ConfirmURL": confirmURL,
	})

	if err != nil {
		return fmt.Errorf("render confirmation email: %w", err)
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", m.fromEmail)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", fmt.Sprintf("Confirm your subscription to %s", repo))
	msg.SetBody("text/html", body)

	return m.sendOne(msg)
}

func (m *Mailer) SendReleaseNotifications(notifications []domain.ReleaseNotification) error {
	if len(notifications) == 0 {
		return nil
	}

	sender, err := m.dialer.Dial()
	if err != nil {
		return fmt.Errorf("dial SMTP: %w", err)
	}

	defer func() {
		if err := sender.Close(); err != nil {
			slog.Error("failed to close SMTP connection", "error", err)
		}
	}()

	var errs []error

	for _, n := range notifications {
		body, err := renderTemplate(releaseTemplate, map[string]string{
			"Repo":           n.Repo,
			"Tag":            n.Tag,
			"ReleaseURL":     n.ReleaseURL,
			"UnsubscribeURL": n.UnsubscribeURL,
		})

		if err != nil {
			errs = append(errs, fmt.Errorf("render email for %s: %w", n.To, err))
			continue
		}

		msg := gomail.NewMessage()
		msg.SetHeader("From", m.fromEmail)
		msg.SetHeader("To", n.To)
		msg.SetHeader("Subject", fmt.Sprintf("New release %s for %s", n.Tag, n.Repo))
		msg.SetBody("text/html", body)

		if err := gomail.Send(sender, msg); err != nil {
			slog.Error("failed to send release notification", "to", n.To, "repo", n.Repo, "error", err)
			errs = append(errs, fmt.Errorf("send email to %s: %w", n.To, err))
		}
	}

	return errors.Join(errs...)
}

func (m *Mailer) sendOne(msg *gomail.Message) error {
	if err := m.dialer.DialAndSend(msg); err != nil {
		return fmt.Errorf("send email: %w", err)
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
