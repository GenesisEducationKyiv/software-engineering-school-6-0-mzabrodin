package notifyrelease

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/domain"
	"github-release-notifier/internal/shared/events"
)

type NotifyReleaseSuite struct {
	suite.Suite

	recipients *mockRecipients
	dedupe     *mockDedupe
	failed     *mockFailedStore
	sender     *mockSender
	urls       *mockURLs
	publisher  *mockPublisher
	uc         *UseCase
	input      Input
}

func TestNotifyReleaseSuite(t *testing.T) {
	suite.Run(t, new(NotifyReleaseSuite))
}

func (s *NotifyReleaseSuite) SetupTest() {
	s.recipients = &mockRecipients{}
	s.dedupe = &mockDedupe{}
	s.failed = &mockFailedStore{}
	s.sender = &mockSender{}
	s.urls = &mockURLs{}
	s.publisher = &mockPublisher{}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s.uc = New(s.recipients, s.dedupe, s.failed, s.sender, s.urls, s.publisher, log)

	s.input = Input{
		SagaID:     uuid.NewString(),
		RepoName:   "golang/go",
		Tag:        "v1.26.0",
		ReleaseURL: "https://github.com/golang/go/releases/tag/v1.26.0",
	}
}

func (s *NotifyReleaseSuite) TearDownTest() {
	s.recipients.AssertExpectations(s.T())
	s.dedupe.AssertExpectations(s.T())
	s.failed.AssertExpectations(s.T())
	s.sender.AssertExpectations(s.T())
	s.publisher.AssertExpectations(s.T())
}

func (s *NotifyReleaseSuite) TestSkipsWhenAlreadyProcessed() {
	s.dedupe.On("Exists", mock.Anything, s.input.RepoName, s.input.Tag).Return(true, nil)

	out, err := s.uc.Execute(s.T().Context(), s.input)

	s.Require().NoError(err)
	s.Equal(0, out.SentCount)
	s.dedupe.AssertNotCalled(s.T(), "Mark")
	s.sender.AssertNotCalled(s.T(), "SendReleaseNotifications")
}

func (s *NotifyReleaseSuite) TestDeliversToAllRecipients() {
	s.dedupe.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	s.dedupe.On("Mark", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	s.recipients.On("RecipientsByRepo", mock.Anything, s.input.RepoName).Return([]domain.Recipient{
		{Email: "a@example.test", UnsubToken: "ta"},
		{Email: "b@example.test", UnsubToken: "tb"},
	}, nil)
	s.urls.On("UnsubscribeURL", mock.Anything).Return("https://example.test/u")
	s.sender.On("SendReleaseNotifications", mock.Anything, mock.Anything).Return(notifier.BatchResult{Sent: 2})
	s.publisher.On("ReleaseSent", mock.Anything, mock.Anything).Return(nil)

	var notified events.ReleaseNotified
	s.publisher.On("ReleaseNotified", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { notified, _ = args.Get(1).(events.ReleaseNotified) }).
		Return(nil)

	out, err := s.uc.Execute(s.T().Context(), s.input)

	s.Require().NoError(err)
	s.Equal(2, out.SentCount)
	s.Empty(out.FailedEmails)
	s.publisher.AssertNumberOfCalls(s.T(), "ReleaseSent", 2)
	s.failed.AssertNotCalled(s.T(), "Add")
	s.Equal(2, notified.SentCount)
	s.Equal(s.input.SagaID, notified.SagaID)
}

func (s *NotifyReleaseSuite) TestRecordsPartialFailure() {
	s.dedupe.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	s.dedupe.On("Mark", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	s.recipients.On("RecipientsByRepo", mock.Anything, mock.Anything).Return([]domain.Recipient{
		{Email: "ok@example.test", UnsubToken: "t1"},
		{Email: "bad@example.test", UnsubToken: "t2"},
	}, nil)
	s.urls.On("UnsubscribeURL", mock.Anything).Return("https://example.test/u")
	s.sender.On("SendReleaseNotifications", mock.Anything, mock.Anything).
		Return(notifier.BatchResult{Sent: 1, Failed: []string{"bad@example.test"}})
	s.publisher.On("ReleaseSent", mock.Anything, mock.Anything).Return(nil)
	s.publisher.On("ReleaseNotified", mock.Anything, mock.Anything).Return(nil)

	var recorded *domain.FailedNotification
	s.failed.On("Add", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { recorded, _ = args.Get(1).(*domain.FailedNotification) }).
		Return(nil)

	var failed events.NotificationReleaseFailed
	s.publisher.On("ReleaseFailed", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { failed, _ = args.Get(1).(events.NotificationReleaseFailed) }).
		Return(nil)

	out, err := s.uc.Execute(s.T().Context(), s.input)

	s.Require().NoError(err)
	s.Equal(1, out.SentCount)
	s.Equal([]string{"bad@example.test"}, out.FailedEmails)
	s.Require().NotNil(recorded)
	s.Equal("bad@example.test", recorded.Email)
	s.Equal(s.input.ReleaseURL, recorded.ReleaseURL)
	s.Equal("bad@example.test", failed.Email)
}

func (s *NotifyReleaseSuite) TestNoRecipientsStillNotifiesAndMarks() {
	s.dedupe.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	s.dedupe.On("Mark", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	s.recipients.On("RecipientsByRepo", mock.Anything, mock.Anything).Return([]domain.Recipient{}, nil)

	var notified events.ReleaseNotified
	s.publisher.On("ReleaseNotified", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { notified, _ = args.Get(1).(events.ReleaseNotified) }).
		Return(nil)

	out, err := s.uc.Execute(s.T().Context(), s.input)

	s.Require().NoError(err)
	s.Equal(0, out.SentCount)
	s.Equal(0, notified.SentCount)
	s.sender.AssertNotCalled(s.T(), "SendReleaseNotifications")
}

func (s *NotifyReleaseSuite) TestPerRecipientPublishFailureIgnored() {
	s.dedupe.On("Exists", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	s.dedupe.On("Mark", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	s.recipients.On("RecipientsByRepo", mock.Anything, mock.Anything).Return([]domain.Recipient{
		{Email: "a@example.test", UnsubToken: "ta"},
	}, nil)
	s.urls.On("UnsubscribeURL", mock.Anything).Return("https://example.test/u")
	s.sender.On("SendReleaseNotifications", mock.Anything, mock.Anything).Return(notifier.BatchResult{Sent: 1})
	s.publisher.On("ReleaseSent", mock.Anything, mock.Anything).Return(errors.New("nats down"))
	s.publisher.On("ReleaseNotified", mock.Anything, mock.Anything).Return(nil)

	out, err := s.uc.Execute(s.T().Context(), s.input)

	s.Require().NoError(err) // per-recipient publish failure is logged, not returned
	s.Equal(1, out.SentCount)
}
