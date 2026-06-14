package notifierconsumer_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"buf.build/go/protovalidate"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"

	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/adapter/notifierconsumer"
	"github-release-notifier/internal/notifier/grpc/gen/notifierv1"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockMailer struct{ mock.Mock }

func (m *mockMailer) SendConfirmation(ctx context.Context, to, repo, confirmURL string) {
	m.Called(ctx, to, repo, confirmURL)
}

func (m *mockMailer) SendReleaseNotifications(
	ctx context.Context,
	notifications []notifier.ReleaseNotification,
) notifier.BatchResult {
	args := m.Called(ctx, notifications)
	out, _ := args.Get(0).(notifier.BatchResult)

	return out
}

type ConsumerSuite struct {
	suite.Suite

	mailer   *mockMailer
	consumer *notifierconsumer.Consumer
}

func TestConsumerSuite(t *testing.T) {
	suite.Run(t, new(ConsumerSuite))
}

func (s *ConsumerSuite) SetupTest() {
	validator, err := protovalidate.New()
	s.Require().NoError(err)

	s.mailer = &mockMailer{}
	s.consumer = notifierconsumer.New(s.mailer, validator, testLogger)
}

func (s *ConsumerSuite) TearDownTest() {
	s.mailer.AssertExpectations(s.T())
}

func (s *ConsumerSuite) marshal(msg proto.Message) []byte {
	data, err := proto.Marshal(msg)
	s.Require().NoError(err)

	return data
}

func (s *ConsumerSuite) TestHandleConfirmationValid() {
	s.mailer.On("SendConfirmation", mock.Anything,
		"user@example.com", "owner/repo", "https://example.com/confirm/abc").Return()

	data := s.marshal(&notifierv1.SendConfirmationRequest{
		To:         "user@example.com",
		Repo:       "owner/repo",
		ConfirmUrl: "https://example.com/confirm/abc",
	})

	s.NoError(s.consumer.HandleConfirmation(context.Background(), data))
}

func (s *ConsumerSuite) TestHandleConfirmationGarbageBytesTerminal() {
	err := s.consumer.HandleConfirmation(context.Background(), []byte{0xFF, 0xFF, 0xFF, 0xFF})

	s.Require().Error(err)
	s.ErrorIs(err, broker.ErrTerminal)
	s.mailer.AssertNotCalled(s.T(), "SendConfirmation")
}

func (s *ConsumerSuite) TestHandleConfirmationInvalidFieldTerminal() {
	data := s.marshal(&notifierv1.SendConfirmationRequest{
		To:         "not-an-email",
		Repo:       "owner/repo",
		ConfirmUrl: "https://example.com/confirm/abc",
	})

	err := s.consumer.HandleConfirmation(context.Background(), data)

	s.Require().Error(err)
	s.ErrorIs(err, broker.ErrTerminal)
	s.mailer.AssertNotCalled(s.T(), "SendConfirmation")
}

func (s *ConsumerSuite) TestHandleReleaseValidAcks() {
	expected := []notifier.ReleaseNotification{{
		To:             "a@example.com",
		Repo:           "owner/repo",
		Tag:            "v1.0.0",
		ReleaseURL:     "https://github.com/owner/repo/releases/tag/v1.0.0",
		UnsubscribeURL: "https://example.com/unsubscribe/a",
	}}

	s.mailer.On("SendReleaseNotifications", mock.Anything, expected).
		Return(notifier.BatchResult{Sent: 1})

	data := s.marshal(&notifierv1.SendReleaseNotificationsRequest{
		Notifications: []*notifierv1.ReleaseNotification{{
			To:             "a@example.com",
			Repo:           "owner/repo",
			Tag:            "v1.0.0",
			ReleaseUrl:     "https://github.com/owner/repo/releases/tag/v1.0.0",
			UnsubscribeUrl: "https://example.com/unsubscribe/a",
		}},
	})

	s.NoError(s.consumer.HandleRelease(context.Background(), data))
}

func (s *ConsumerSuite) TestHandleReleaseAllFailedRetryableError() {
	s.mailer.On("SendReleaseNotifications", mock.Anything, mock.Anything).
		Return(notifier.BatchResult{Failed: []string{"a@example.com"}})

	data := s.marshal(&notifierv1.SendReleaseNotificationsRequest{
		Notifications: []*notifierv1.ReleaseNotification{{
			To:             "a@example.com",
			Repo:           "owner/repo",
			Tag:            "v1.0.0",
			ReleaseUrl:     "https://github.com/owner/repo/releases/tag/v1.0.0",
			UnsubscribeUrl: "https://example.com/unsubscribe/a",
		}},
	})

	err := s.consumer.HandleRelease(context.Background(), data)

	s.Require().Error(err)
	s.NotErrorIs(err, broker.ErrTerminal)
}

func (s *ConsumerSuite) TestHandleReleaseEmptyNotificationsTerminal() {
	data := s.marshal(&notifierv1.SendReleaseNotificationsRequest{})

	err := s.consumer.HandleRelease(context.Background(), data)

	s.Require().Error(err)
	s.ErrorIs(err, broker.ErrTerminal)
	s.mailer.AssertNotCalled(s.T(), "SendReleaseNotifications")
}
