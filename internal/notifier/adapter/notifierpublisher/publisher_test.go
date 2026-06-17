package notifierpublisher_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"

	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/adapter/notifierpublisher"
	"github-release-notifier/internal/notifier/grpc/gen/notifierv1"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockPublisher struct{ mock.Mock }

func (m *mockPublisher) Publish(ctx context.Context, subject string, data []byte) error {
	args := m.Called(ctx, subject, data)

	return args.Error(0)
}

type PublisherSuite struct {
	suite.Suite

	pub       *mockPublisher
	publisher *notifierpublisher.Publisher
}

func TestPublisherSuite(t *testing.T) {
	suite.Run(t, new(PublisherSuite))
}

func (s *PublisherSuite) SetupTest() {
	s.pub = &mockPublisher{}
	s.publisher = notifierpublisher.New(s.pub, testLogger)
}

func (s *PublisherSuite) TearDownTest() {
	s.pub.AssertExpectations(s.T())
}

func (s *PublisherSuite) capturePublishedData(captured *[]byte) func(mock.Arguments) {
	return func(args mock.Arguments) {
		data, _ := args.Get(2).([]byte)
		*captured = data
	}
}

func (s *PublisherSuite) TestSendReleaseNotificationsPublishesAndReportsSent() {
	var captured []byte
	s.pub.On("Publish", mock.Anything, notifier.SubjectRelease, mock.Anything).
		Run(s.capturePublishedData(&captured)).
		Return(nil)

	notifications := []notifier.ReleaseNotification{
		{
			To:             "a@example.com",
			Repo:           "owner/repo",
			Tag:            "v1",
			ReleaseURL:     "https://x/1",
			UnsubscribeURL: "https://x/u/a",
		},
		{
			To:             "b@example.com",
			Repo:           "owner/repo",
			Tag:            "v1",
			ReleaseURL:     "https://x/1",
			UnsubscribeURL: "https://x/u/b",
		},
	}

	result := s.publisher.SendReleaseNotifications(context.Background(), notifications)

	s.Equal(2, result.Sent)
	s.Empty(result.Failed)

	var got notifierv1.SendReleaseNotificationsRequest
	s.Require().NoError(proto.Unmarshal(captured, &got))
	s.Len(got.GetNotifications(), 2)
	s.Equal("a@example.com", got.GetNotifications()[0].GetTo())
}

func (s *PublisherSuite) TestSendReleaseNotificationsPublishErrorAllFailed() {
	s.pub.On("Publish", mock.Anything, notifier.SubjectRelease, mock.Anything).
		Return(errors.New("broker down"))

	notifications := []notifier.ReleaseNotification{
		{
			To:             "a@example.com",
			Repo:           "owner/repo",
			Tag:            "v1",
			ReleaseURL:     "https://x/1",
			UnsubscribeURL: "https://x/u/a",
		},
	}

	result := s.publisher.SendReleaseNotifications(context.Background(), notifications)

	s.Zero(result.Sent)
	s.Equal([]string{"a@example.com"}, result.Failed)
}

func (s *PublisherSuite) TestSendConfirmationPublishesConfirmation() {
	var captured []byte
	s.pub.On("Publish", mock.Anything, notifier.SubjectConfirmation, mock.Anything).
		Run(s.capturePublishedData(&captured)).
		Return(nil)

	s.publisher.SendConfirmation(
		context.Background(),
		"user@example.com",
		"owner/repo",
		"https://example.com/confirm/abc",
	)
	s.Require().NoError(s.publisher.Close())

	var got notifierv1.SendConfirmationRequest
	s.Require().NoError(proto.Unmarshal(captured, &got))
	s.Equal("user@example.com", got.GetTo())
	s.Equal("owner/repo", got.GetRepo())
	s.Equal("https://example.com/confirm/abc", got.GetConfirmUrl())
}
