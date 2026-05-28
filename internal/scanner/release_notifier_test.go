package scanner_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/domain"
	"github-release-notifier/internal/scanner"
)

var ctx = context.Background()

type mockMailer struct{ mock.Mock }

func (m *mockMailer) SendReleaseNotifications(ns []domain.ReleaseNotification) error {
	return m.Called(ns).Error(0)
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
	repo := &domain.Repository{Name: "owner/repo"}
	release := &domain.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"}
	subs := []*domain.Subscription{
		{Email: "a@example.com", UnsubscribeToken: "tok-a"},
		{Email: "b@example.com", UnsubscribeToken: "tok-b"},
	}

	u := &mockURLBuilder{}
	u.On("UnsubscribeURL", "tok-a").Return("https://example.com/unsubscribe/tok-a")
	u.On("UnsubscribeURL", "tok-b").Return("https://example.com/unsubscribe/tok-b")
	defer u.AssertExpectations(s.T())

	m := &mockMailer{}
	var capturedNotifications []domain.ReleaseNotification
	m.On("SendReleaseNotifications", mock.Anything).
		Run(func(args mock.Arguments) {
			capturedNotifications, _ = args.Get(0).([]domain.ReleaseNotification)
		}).Return(nil)
	defer m.AssertExpectations(s.T())

	n := scanner.NewReleaseNotifier(m, u)
	s.Require().NoError(n.Notify(ctx, subs, repo, release))

	s.Require().Len(capturedNotifications, 2)
	s.Equal("a@example.com", capturedNotifications[0].To)
	s.Equal("owner/repo", capturedNotifications[0].Repo)
	s.Equal("v1.0.0", capturedNotifications[0].Tag)
	s.Equal(release.HTMLURL, capturedNotifications[0].ReleaseURL)
	s.Equal("https://example.com/unsubscribe/tok-a", capturedNotifications[0].UnsubscribeURL)
	s.Equal("b@example.com", capturedNotifications[1].To)
	s.Equal("https://example.com/unsubscribe/tok-b", capturedNotifications[1].UnsubscribeURL)
}

func (s *ReleaseNotifierSuite) TestUnsubscribeURLCalledPerSub() {
	repo := &domain.Repository{Name: "owner/repo"}
	release := &domain.Release{TagName: "v1.0.0", HTMLURL: "..."}
	subs := []*domain.Subscription{
		{Email: "a@example.com", UnsubscribeToken: "tok-a"},
		{Email: "b@example.com", UnsubscribeToken: "tok-b"},
	}

	u := &mockURLBuilder{}
	u.On("UnsubscribeURL", "tok-a").Return("https://example.com/unsubscribe/tok-a").Once()
	u.On("UnsubscribeURL", "tok-b").Return("https://example.com/unsubscribe/tok-b").Once()
	defer u.AssertExpectations(s.T())

	m := &mockMailer{}
	m.On("SendReleaseNotifications", mock.Anything).Return(nil)

	n := scanner.NewReleaseNotifier(m, u)
	s.Require().NoError(n.Notify(ctx, subs, repo, release))
}

func (s *ReleaseNotifierSuite) TestMailerError_Propagated() {
	repo := &domain.Repository{Name: "owner/repo"}
	release := &domain.Release{TagName: "v1.0.0", HTMLURL: "..."}
	subs := []*domain.Subscription{{Email: "a@example.com", UnsubscribeToken: "tok"}}

	u := &mockURLBuilder{}
	u.On("UnsubscribeURL", "tok").Return("https://example.com/unsubscribe/tok")

	m := &mockMailer{}
	m.On("SendReleaseNotifications", mock.Anything).Return(errors.New("smtp error"))
	defer m.AssertExpectations(s.T())

	n := scanner.NewReleaseNotifier(m, u)
	s.Error(n.Notify(ctx, subs, repo, release))
}
