//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"buf.build/go/protovalidate"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/adapter/notifierconsumer"
	"github-release-notifier/internal/notifier/adapter/notifierpublisher"
)

type mockNotifierMailer struct{ mock.Mock }

func (m *mockNotifierMailer) SendConfirmation(ctx context.Context, to, repo, confirmURL string) {
	m.Called(ctx, to, repo, confirmURL)
}

func (m *mockNotifierMailer) SendReleaseNotifications(
	ctx context.Context,
	notifications []notifier.ReleaseNotification,
) notifier.BatchResult {
	args := m.Called(ctx, notifications)
	out, _ := args.Get(0).(notifier.BatchResult)

	return out
}

type NotifierNATSSuite struct {
	suite.Suite

	conn   *broker.Conn
	mailer *mockNotifierMailer
	pub    *notifierpublisher.Publisher
	stops  []func()
}

func TestNotifierNATSSuite(t *testing.T) {
	suite.Run(t, new(NotifierNATSSuite))
}

func (s *NotifierNATSSuite) SetupTest() {
	conn, err := broker.Connect(testNATSURL, testLogger)
	s.Require().NoError(err)
	s.conn = conn

	s.Require().NoError(conn.EnsureStream(s.T().Context(), notifier.StreamEmail,
		[]string{notifier.SubjectConfirmation, notifier.SubjectRelease}))

	validator, err := protovalidate.New()
	s.Require().NoError(err)

	s.mailer = &mockNotifierMailer{}
	consumer := notifierconsumer.New(s.mailer, validator, testLogger)

	stopConfirm, err := conn.Consume(s.T().Context(), notifier.StreamEmail, "test-confirmation",
		notifier.SubjectConfirmation, consumer.HandleConfirmation)
	s.Require().NoError(err)

	stopRelease, err := conn.Consume(s.T().Context(), notifier.StreamEmail, "test-release",
		notifier.SubjectRelease, consumer.HandleRelease)
	s.Require().NoError(err)

	s.stops = []func(){stopConfirm, stopRelease}
	s.pub = notifierpublisher.New(conn, testLogger)
}

func (s *NotifierNATSSuite) TearDownTest() {
	s.Require().NoError(s.pub.Close())
	for _, stop := range s.stops {
		stop()
	}
	s.Require().NoError(s.conn.Close())
	s.mailer.AssertExpectations(s.T())
}

func signalOnCall(called chan<- struct{}) func(mock.Arguments) {
	return func(mock.Arguments) {
		select {
		case called <- struct{}{}:
		default:
		}
	}
}

func (s *NotifierNATSSuite) waitForCall(called <-chan struct{}, msg string) {
	s.Require().Eventually(func() bool {
		select {
		case <-called:
			return true
		default:
			return false
		}
	}, 5*time.Second, 50*time.Millisecond, msg)
}

func (s *NotifierNATSSuite) TestConfirmationRoundTrip() {
	called := make(chan struct{}, 1)
	s.mailer.On("SendConfirmation", mock.Anything,
		"user@example.com", "owner/repo", "https://example.com/confirm/abc").
		Return().
		Run(signalOnCall(called))

	s.pub.SendConfirmation(s.T().Context(), "user@example.com", "owner/repo", "https://example.com/confirm/abc")

	s.waitForCall(called, "confirmation command was not consumed")
}

func (s *NotifierNATSSuite) TestReleaseRoundTrip() {
	notifications := []notifier.ReleaseNotification{{
		To:             "a@example.com",
		Repo:           "owner/repo",
		Tag:            "v1.0.0",
		ReleaseURL:     "https://github.com/owner/repo/releases/tag/v1.0.0",
		UnsubscribeURL: "https://example.com/unsubscribe/a",
	}}

	called := make(chan struct{}, 1)
	s.mailer.On("SendReleaseNotifications", mock.Anything, notifications).
		Return(notifier.BatchResult{Sent: 1}).
		Run(signalOnCall(called))

	result := s.pub.SendReleaseNotifications(s.T().Context(), notifications)
	s.Equal(1, result.Sent)

	s.waitForCall(called, "release command was not consumed")
}
