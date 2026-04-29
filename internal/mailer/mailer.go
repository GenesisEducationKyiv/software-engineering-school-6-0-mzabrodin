package mailer

import (
	"fmt"
	"log/slog"

	"github-release-notifier/internal/domain"

	"gopkg.in/gomail.v2"
)

type Mailer struct {
	host      string
	port      int
	user      string
	password  string
	fromEmail string
}

func NewMailer(host string, port int, user, password, fromEmail string) *Mailer {
	return &Mailer{
		host:      host,
		port:      port,
		user:      user,
		password:  password,
		fromEmail: fromEmail,
	}
}

func (m *Mailer) SendConfirmation(to, repo, confirmURL string) error {
	msg := gomail.NewMessage()
	msg.SetHeader("From", m.fromEmail)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", fmt.Sprintf("Confirm your subscription to %s", repo))
	msg.SetBody("text/html", confirmationBody(repo, confirmURL))

	return m.sendOne(msg)
}

func (m *Mailer) SendReleaseNotifications(notifications []domain.ReleaseNotification) error {
	if len(notifications) == 0 {
		return nil
	}

	dialer := gomail.NewDialer(m.host, m.port, m.user, m.password)

	sender, err := dialer.Dial()
	if err != nil {
		return fmt.Errorf("dial SMTP: %w", err)
	}

	defer func(sender gomail.SendCloser) {
		err := sender.Close()
		if err != nil {
			slog.Error("failed to close SMTP connection", "error", err)
		}
	}(sender)

	for _, n := range notifications {
		msg := gomail.NewMessage()
		msg.SetHeader("From", m.fromEmail)
		msg.SetHeader("To", n.To)
		msg.SetHeader("Subject", fmt.Sprintf("New release %s for %s", n.Tag, n.Repo))
		msg.SetBody("text/html", releaseBody(n.Repo, n.Tag, n.ReleaseURL, n.UnsubscribeURL))

		if err := gomail.Send(sender, msg); err != nil {
			return fmt.Errorf("send email to %s: %w", n.To, err)
		}
	}

	return nil
}

func (m *Mailer) sendOne(msg *gomail.Message) error {
	dialer := gomail.NewDialer(m.host, m.port, m.user, m.password)
	if err := dialer.DialAndSend(msg); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

func confirmationBody(repo, confirmURL string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body>
	<h2>Confirm your subscription</h2>
	<p>You requested to subscribe to new releases of <strong>%s</strong>.</p>
	<p>Click <a href="%s">here</a> to confirm your subscription.</p>
	<p>If you did not request this, please ignore this email.</p>
</body>
</html>
`, repo, confirmURL)
}

func releaseBody(repo, tag, releaseURL, unsubscribeURL string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body>
	<h2>New release for %s</h2>
	<p>A new release <strong>%s</strong> is available.</p>
	<p><a href="%s">View</a> release on GitHub</p>
	<br>
	<p><small><a href="%s">Unsubscribe</a></small></p>
</body>
</html>
`, repo, tag, releaseURL, unsubscribeURL)
}
