package mailer

import (
	"strings"
	"testing"

	"github-release-notifier/internal/domain"
)

func TestConfirmationTemplate(t *testing.T) {
	body, err := renderTemplate(confirmationTemplate, map[string]string{
		"Repo":       "golang/go",
		"ConfirmURL": "http://localhost/confirm/abc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{"golang/go", "http://localhost/confirm/abc", "<html>"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestReleaseTemplate(t *testing.T) {
	body, err := renderTemplate(releaseTemplate, map[string]string{
		"Repo":           "owner/repo",
		"Tag":            "v2.0.0",
		"ReleaseURL":     "http://github.com/release",
		"UnsubscribeURL": "http://example.com/unsub",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{
		"owner/repo",
		"v2.0.0",
		"http://github.com/release",
		"http://example.com/unsub",
		"<html>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestSendReleaseNotifications_EmptySlice_NoError(t *testing.T) {
	m := NewMailer("localhost", 587, "user", "pass", "from@example.com")
	err := m.SendReleaseNotifications([]domain.ReleaseNotification{})

	if err != nil {
		t.Errorf("expected nil for empty notifications, got %v", err)
	}
}
