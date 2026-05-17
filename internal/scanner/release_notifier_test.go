package scanner

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github-release-notifier/internal/domain"
)

type mockMailer struct {
	called        bool
	notifications []domain.ReleaseNotification
	err           error
}

func (m *mockMailer) SendReleaseNotifications(ns []domain.ReleaseNotification) error {
	m.called = true
	m.notifications = ns
	return m.err
}

type mockURLBuilder struct {
	calls []string
}

func (m *mockURLBuilder) UnsubscribeURL(token string) string {
	m.calls = append(m.calls, token)
	return "https://example.com/unsubscribe/" + token
}

func TestReleaseNotifier_BuildsCorrectNotifications(t *testing.T) {
	repo := testRepo("owner/repo", nil)
	release := &domain.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"}
	subs := []*domain.Subscription{
		{Email: "a@example.com", UnsubscribeToken: "tok-a"},
		{Email: "b@example.com", UnsubscribeToken: "tok-b"},
	}

	m := &mockMailer{}
	n := NewReleaseNotifier(m, &mockURLBuilder{})

	require.NoError(t, n.Notify(ctx, subs, repo, release))
	require.Len(t, m.notifications, 2)

	assert.Equal(t, "a@example.com", m.notifications[0].To)
	assert.Equal(t, "owner/repo", m.notifications[0].Repo)
	assert.Equal(t, "v1.0.0", m.notifications[0].Tag)
	assert.Equal(t, release.HTMLURL, m.notifications[0].ReleaseURL)
	assert.Equal(t, "https://example.com/unsubscribe/tok-a", m.notifications[0].UnsubscribeURL)

	assert.Equal(t, "b@example.com", m.notifications[1].To)
	assert.Equal(t, "https://example.com/unsubscribe/tok-b", m.notifications[1].UnsubscribeURL)
}

func TestReleaseNotifier_UnsubscribeURLCalledPerSub(t *testing.T) {
	repo := testRepo("owner/repo", nil)
	release := &domain.Release{TagName: "v1.0.0", HTMLURL: "..."}
	subs := []*domain.Subscription{
		{Email: "a@example.com", UnsubscribeToken: "tok-a"},
		{Email: "b@example.com", UnsubscribeToken: "tok-b"},
	}

	u := &mockURLBuilder{}
	n := NewReleaseNotifier(&mockMailer{}, u)

	require.NoError(t, n.Notify(ctx, subs, repo, release))
	assert.Equal(t, []string{"tok-a", "tok-b"}, u.calls)
}

func TestReleaseNotifier_MailerError_Propagated(t *testing.T) {
	repo := testRepo("owner/repo", nil)
	release := &domain.Release{TagName: "v1.0.0", HTMLURL: "..."}
	subs := []*domain.Subscription{{Email: "a@example.com", UnsubscribeToken: "tok"}}
	n := NewReleaseNotifier(&mockMailer{err: errors.New("smtp error")}, &mockURLBuilder{})
	assert.Error(t, n.Notify(ctx, subs, repo, release))
}
