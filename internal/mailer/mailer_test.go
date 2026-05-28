package mailer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/domain"
)

type MailerSuite struct {
	suite.Suite
}

func TestMailerSuite(t *testing.T) {
	suite.Run(t, new(MailerSuite))
}

func (s *MailerSuite) TestConfirmationTemplate() {
	body, err := renderTemplate(confirmationTemplate, map[string]string{
		"Repo":       "golang/go",
		"ConfirmURL": "http://localhost/confirm/abc",
	})
	s.Require().NoError(err)
	s.Contains(body, "golang/go")
	s.Contains(body, "http://localhost/confirm/abc")
	s.Contains(body, "<html>")
}

func (s *MailerSuite) TestReleaseTemplate() {
	body, err := renderTemplate(releaseTemplate, map[string]string{
		"Repo":           "owner/repo",
		"Tag":            "v2.0.0",
		"ReleaseURL":     "http://github.com/release",
		"UnsubscribeURL": "http://example.com/unsub",
	})
	s.Require().NoError(err)
	s.Contains(body, "owner/repo")
	s.Contains(body, "v2.0.0")
	s.Contains(body, "http://github.com/release")
	s.Contains(body, "http://example.com/unsub")
	s.Contains(body, "<html>")
}

func (s *MailerSuite) TestSendReleaseNotifications_EmptySlice_NoError() {
	m, err := NewMailer("localhost", 587, "user", "pass", "from@example.com")
	s.Require().NoError(err)
	s.NoError(m.SendReleaseNotifications(context.Background(), []domain.ReleaseNotification{}))
}
