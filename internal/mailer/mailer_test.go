package mailer

import (
	"github-release-notifier/internal/domain"
	"strings"
	"testing"
)

func TestConfirmationBody_ContainsRepo(t *testing.T) {
	body := confirmationBody("golang/go", "http://localhost/confirm/abc")
	if !strings.Contains(body, "golang/go") {
		t.Error("confirmation body missing repo name")
	}
}

func TestConfirmationBody_ContainsConfirmURL(t *testing.T) {
	url := "http://localhost/confirm/abc123"
	body := confirmationBody("golang/go", url)
	if !strings.Contains(body, url) {
		t.Error("confirmation body missing confirm URL")
	}
}

func TestConfirmationBody_IsHTML(t *testing.T) {
	body := confirmationBody("owner/repo", "http://example.com")
	if !strings.Contains(body, "<html>") {
		t.Error("confirmation body should be HTML")
	}
}

func TestReleaseBody_ContainsRepo(t *testing.T) {
	body := releaseBody("owner/repo", "v2.0.0", "http://github.com/release", "http://example.com/unsub")
	if !strings.Contains(body, "owner/repo") {
		t.Error("release body missing repo name")
	}
}

func TestReleaseBody_ContainsTag(t *testing.T) {
	body := releaseBody("owner/repo", "v2.0.0", "http://github.com/release", "http://example.com/unsub")
	if !strings.Contains(body, "v2.0.0") {
		t.Error("release body missing tag")
	}
}

func TestReleaseBody_ContainsReleaseURL(t *testing.T) {
	releaseURL := "http://github.com/owner/repo/releases/tag/v2.0.0"
	body := releaseBody("owner/repo", "v2.0.0", releaseURL, "http://example.com/unsub")
	if !strings.Contains(body, releaseURL) {
		t.Error("release body missing release URL")
	}
}

func TestReleaseBody_ContainsUnsubscribeURL(t *testing.T) {
	unsubURL := "http://localhost/unsubscribe/tok123"
	body := releaseBody("owner/repo", "v2.0.0", "http://github.com/release", unsubURL)
	if !strings.Contains(body, unsubURL) {
		t.Error("release body missing unsubscribe URL")
	}
}

func TestReleaseBody_IsHTML(t *testing.T) {
	body := releaseBody("owner/repo", "v1.0.0", "http://example.com", "http://example.com/unsub")
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
