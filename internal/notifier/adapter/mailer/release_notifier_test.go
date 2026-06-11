package mailer_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/adapter/mailer"
	"github-release-notifier/internal/shared/entity"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockReleaseSender struct{ mock.Mock }

func (m *mockReleaseSender) SendReleaseNotifications(
	_ context.Context,
	ns []notifier.ReleaseNotification,
) notifier.BatchResult {
	args := m.Called(ns)
	res, _ := args.Get(0).(notifier.BatchResult)
	return res
}

type mockURLBuilder struct{ mock.Mock }

func (m *mockURLBuilder) UnsubscribeURL(token string) string {
	return m.Called(token).String(0)
}

type ReleaseNotifierSuite struct {
	suite.Suite
}

func TestReleaseNotifierSuite(t *testing.T) {
	suite.Run(t, new(ReleaseNotifierSuite))
}

func (s *ReleaseNotifierSuite) TestBuildsCorrectNotifications() {
	repo := &entity.Repository{Name: "owner/repo"}
	release := &entity.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"}
	subs := []*entity.Subscription{
		{Email: "a@example.com", UnsubscribeToken: "tok-a"},
		{Email: "b@example.com", UnsubscribeToken: "tok-b"},
	}

	u := &mockURLBuilder{}
	u.On("UnsubscribeURL", "tok-a").Return("https://example.com/unsubscribe/tok-a")
	u.On("UnsubscribeURL", "tok-b").Return("https://example.com/unsubscribe/tok-b")
	defer u.AssertExpectations(s.T())

	m := &mockReleaseSender{}
	var captured []notifier.ReleaseNotification
	m.On("SendReleaseNotifications", mock.Anything).
		Run(func(args mock.Arguments) {
			captured, _ = args.Get(0).([]notifier.ReleaseNotification)
		}).Return(notifier.BatchResult{Sent: 2})
	defer m.AssertExpectations(s.T())

	n := mailer.NewReleaseNotifier(m, u, testLogger)
	s.Require().NoError(n.Notify(s.T().Context(), subs, repo, release))

	s.Require().Len(captured, 2)
	s.Equal("a@example.com", captured[0].To)
	s.Equal("owner/repo", captured[0].Repo)
	s.Equal("v1.0.0", captured[0].Tag)
	s.Equal(release.HTMLURL, captured[0].ReleaseURL)
	s.Equal("https://example.com/unsubscribe/tok-a", captured[0].UnsubscribeURL)
	s.Equal("b@example.com", captured[1].To)
	s.Equal("https://example.com/unsubscribe/tok-b", captured[1].UnsubscribeURL)
}

func (s *ReleaseNotifierSuite) TestUnsubscribeURLCalledPerSub() {
	repo := &entity.Repository{Name: "owner/repo"}
	release := &entity.Release{TagName: "v1.0.0", HTMLURL: "..."}
	subs := []*entity.Subscription{
		{Email: "a@example.com", UnsubscribeToken: "tok-a"},
		{Email: "b@example.com", UnsubscribeToken: "tok-b"},
	}

	u := &mockURLBuilder{}
	u.On("UnsubscribeURL", "tok-a").Return("https://example.com/unsubscribe/tok-a").Once()
	u.On("UnsubscribeURL", "tok-b").Return("https://example.com/unsubscribe/tok-b").Once()
	defer u.AssertExpectations(s.T())

	m := &mockReleaseSender{}
	m.On("SendReleaseNotifications", mock.Anything).Return(notifier.BatchResult{Sent: 2})

	n := mailer.NewReleaseNotifier(m, u, testLogger)
	s.Require().NoError(n.Notify(s.T().Context(), subs, repo, release))
}

func (s *ReleaseNotifierSuite) TestAllFailed_ReturnsError() {
	repo := &entity.Repository{Name: "owner/repo"}
	release := &entity.Release{TagName: "v1.0.0", HTMLURL: "..."}
	subs := []*entity.Subscription{{Email: "a@example.com", UnsubscribeToken: "tok"}}

	u := &mockURLBuilder{}
	u.On("UnsubscribeURL", "tok").Return("https://example.com/unsubscribe/tok")

	m := &mockReleaseSender{}
	m.On("SendReleaseNotifications", mock.Anything).
		Return(notifier.BatchResult{Failed: []string{"a@example.com"}})
	defer m.AssertExpectations(s.T())

	n := mailer.NewReleaseNotifier(m, u, testLogger)
	s.Error(n.Notify(s.T().Context(), subs, repo, release))
}

func (s *ReleaseNotifierSuite) TestPartialFailure_NoError() {
	repo := &entity.Repository{Name: "owner/repo"}
	release := &entity.Release{TagName: "v1.0.0", HTMLURL: "..."}
	subs := []*entity.Subscription{
		{Email: "a@example.com", UnsubscribeToken: "tok-a"},
		{Email: "b@example.com", UnsubscribeToken: "tok-b"},
	}

	u := &mockURLBuilder{}
	u.On("UnsubscribeURL", "tok-a").Return("https://example.com/unsubscribe/tok-a")
	u.On("UnsubscribeURL", "tok-b").Return("https://example.com/unsubscribe/tok-b")

	m := &mockReleaseSender{}
	m.On("SendReleaseNotifications", mock.Anything).
		Return(notifier.BatchResult{Sent: 1, Failed: []string{"b@example.com"}})
	defer m.AssertExpectations(s.T())

	n := mailer.NewReleaseNotifier(m, u, testLogger)
	s.NoError(n.Notify(s.T().Context(), subs, repo, release))
}
