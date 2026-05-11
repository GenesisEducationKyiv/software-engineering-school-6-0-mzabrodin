package mailer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github-release-notifier/internal/domain"
)

func TestConfirmationTemplate(t *testing.T) {
	body, err := renderTemplate(confirmationTemplate, map[string]string{
		"Repo":       "golang/go",
		"ConfirmURL": "http://localhost/confirm/abc",
	})
	require.NoError(t, err)

	assert.Contains(t, body, "golang/go")
	assert.Contains(t, body, "http://localhost/confirm/abc")
	assert.Contains(t, body, "<html>")
}

func TestReleaseTemplate(t *testing.T) {
	body, err := renderTemplate(releaseTemplate, map[string]string{
		"Repo":           "owner/repo",
		"Tag":            "v2.0.0",
		"ReleaseURL":     "http://github.com/release",
		"UnsubscribeURL": "http://example.com/unsub",
	})
	require.NoError(t, err)

	assert.Contains(t, body, "owner/repo")
	assert.Contains(t, body, "v2.0.0")
	assert.Contains(t, body, "http://github.com/release")
	assert.Contains(t, body, "http://example.com/unsub")
	assert.Contains(t, body, "<html>")
}

func TestSendReleaseNotifications_EmptySlice_NoError(t *testing.T) {
	m := NewMailer("localhost", 587, "user", "pass", "from@example.com")
	assert.NoError(t, m.SendReleaseNotifications([]domain.ReleaseNotification{}))
}
