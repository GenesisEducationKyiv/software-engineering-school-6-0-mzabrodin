package mailer

import (
	"strings"
	"testing"

	"github-release-notifier/internal/domain"
)

func TestConfirmationTemplate_ContainsRepo(t *testing.T) {
	body, err := renderTemplate(confirmationTemplate, map[string]string{
		"Repo":       "golang/go",
		"ConfirmURL": "http://localhost/confirm/abc",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(body, "golang/go") {
		t.Error("confirmation body missing repo name")
	}
}

func TestConfirmationTemplate_ContainsConfirmURL(t *testing.T) {
	url := "http://localhost/confirm/abc123"
	body, err := renderTemplate(confirmationTemplate, map[string]string{
		"Repo":       "golang/go",
		"ConfirmURL": url,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(body, url) {
		t.Error("confirmation body missing confirm URL")
	}
}

func TestConfirmationTemplate_IsHTML(t *testing.T) {
	body, err := renderTemplate(confirmationTemplate, map[string]string{
		"Repo":       "owner/repo",
		"ConfirmURL": "http://example.com",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "<html>") {
		t.Error("confirmation body should be HTML")
	}
}

func TestReleaseTemplate_ContainsRepo(t *testing.T) {
	body, err := renderTemplate(releaseTemplate, map[string]string{
		"Repo":           "owner/repo",
		"Tag":            "v2.0.0",
		"ReleaseURL":     "http://github.com/release",
		"UnsubscribeURL": "http://example.com/unsub",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(body, "owner/repo") {
		t.Error("release body missing repo name")
	}
}

func TestReleaseTemplate_ContainsTag(t *testing.T) {
	body, err := renderTemplate(releaseTemplate, map[string]string{
		"Repo":           "owner/repo",
		"Tag":            "v2.0.0",
		"ReleaseURL":     "http://github.com/release",
		"UnsubscribeURL": "http://example.com/unsub",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(body, "v2.0.0") {
		t.Error("release body missing tag")
	}
}

func TestReleaseTemplate_ContainsReleaseURL(t *testing.T) {
	releaseURL := "http://github.com/owner/repo/releases/tag/v2.0.0"
	body, err := renderTemplate(releaseTemplate, map[string]string{
		"Repo":           "owner/repo",
		"Tag":            "v2.0.0",
		"ReleaseURL":     releaseURL,
		"UnsubscribeURL": "http://example.com/unsub",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(body, releaseURL) {
		t.Error("release body missing release URL")
	}
}

func TestReleaseTemplate_ContainsUnsubscribeURL(t *testing.T) {
	unsubURL := "http://localhost/unsubscribe/tok123"
	body, err := renderTemplate(releaseTemplate, map[string]string{
		"Repo":           "owner/repo",
		"Tag":            "v2.0.0",
		"ReleaseURL":     "http://github.com/release",
		"UnsubscribeURL": unsubURL,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(body, unsubURL) {
		t.Error("release body missing unsubscribe URL")
	}
}

func TestReleaseTemplate_IsHTML(t *testing.T) {
	body, err := renderTemplate(releaseTemplate, map[string]string{
		"Repo":           "owner/repo",
		"Tag":            "v1.0.0",
		"ReleaseURL":     "http://example.com",
		"UnsubscribeURL": "http://example.com/unsub",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(body, "<html>") {
		t.Error("release body should be HTML")
	}
}

func TestSendReleaseNotifications_EmptySlice_NoError(t *testing.T) {
	m := NewMailer("localhost", 587, "user", "pass", "from@example.com")
	err := m.SendReleaseNotifications([]domain.ReleaseNotification{})

	if err != nil {
		t.Errorf("expected nil for empty notifications, got %v", err)
	}
}
