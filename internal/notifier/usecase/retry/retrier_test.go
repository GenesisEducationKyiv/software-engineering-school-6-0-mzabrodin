package retry

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/domain"
	shareddomain "github-release-notifier/internal/shared/domain"
)

const maxRetries = 3

type RetrySuite struct {
	suite.Suite

	notifications *mockFailedNotifications
	confirmations *mockFailedConfirmations
	recipients    *mockRecipients
	relSender     *mockReleaseSender
	confSender    *mockConfirmationSender
	urls          *mockURLs
	publisher     *mockPublisher
	retrier       *Retrier
}

func TestRetrySuite(t *testing.T) {
	suite.Run(t, new(RetrySuite))
}

func (s *RetrySuite) SetupTest() {
	s.notifications = &mockFailedNotifications{}
	s.confirmations = &mockFailedConfirmations{}
	s.recipients = &mockRecipients{}
	s.relSender = &mockReleaseSender{}
	s.confSender = &mockConfirmationSender{}
	s.urls = &mockURLs{}
	s.publisher = &mockPublisher{}
	s.publisher.On("Notify").Return().Maybe()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s.retrier = New(
		s.notifications, s.confirmations, s.recipients, s.relSender, s.confSender, s.urls,
		mockTransactor{}, s.publisher,
		Config{MaxRetries: maxRetries, ConfirmationTTL: 24 * time.Hour}, log,
	)
}

func (s *RetrySuite) TearDownTest() {
	s.notifications.AssertExpectations(s.T())
	s.confirmations.AssertExpectations(s.T())
	s.recipients.AssertExpectations(s.T())
	s.relSender.AssertExpectations(s.T())
	s.confSender.AssertExpectations(s.T())
	s.publisher.AssertExpectations(s.T())
}

func (s *RetrySuite) failedNotification(retryCount int) domain.FailedNotification {
	return domain.FailedNotification{
		ID:         1,
		SagaID:     uuid.NewString(),
		RepoName:   "golang/go",
		Tag:        "v1.26.0",
		ReleaseURL: "https://github.com/golang/go/releases/tag/v1.26.0",
		Email:      "user@example.test",
		RetryCount: retryCount,
	}
}

func (s *RetrySuite) failedConfirmation(retryCount int) domain.FailedConfirmation {
	return domain.FailedConfirmation{
		ID:         1,
		SagaID:     uuid.NewString(),
		Email:      "user@example.test",
		RepoName:   "golang/go",
		ConfirmURL: "https://example.test/api/confirm/token",
		RetryCount: retryCount,
	}
}

func (s *RetrySuite) TestReleaseRetrySucceeds() {
	s.notifications.On("ListRetryable", mock.Anything, maxRetries).
		Return([]domain.FailedNotification{s.failedNotification(0)}, nil)
	s.recipients.On("Recipient", mock.Anything, "user@example.test", "golang/go").
		Return(domain.Recipient{Email: "user@example.test", UnsubToken: "t"}, nil)
	s.urls.On("UnsubscribeURL", mock.Anything).Return("https://example.test/u")
	s.relSender.On("SendReleaseNotifications", mock.Anything, mock.Anything).Return(notifier.BatchResult{Sent: 1})
	s.notifications.On("Delete", mock.Anything, int64(1)).Return(nil)
	s.publisher.On("ReleaseSent", mock.Anything, mock.Anything).Return(nil)

	s.Require().NoError(s.retrier.Releases(s.T().Context()))
	s.notifications.AssertNotCalled(s.T(), "IncrementRetry")
}

func (s *RetrySuite) TestReleaseRetryDeadLettersOnFinalAttempt() {
	s.notifications.On("ListRetryable", mock.Anything, maxRetries).
		Return([]domain.FailedNotification{s.failedNotification(maxRetries - 1)}, nil)
	s.recipients.On("Recipient", mock.Anything, mock.Anything, mock.Anything).
		Return(domain.Recipient{Email: "user@example.test", UnsubToken: "t"}, nil)
	s.urls.On("UnsubscribeURL", mock.Anything).Return("https://example.test/u")
	s.relSender.On("SendReleaseNotifications", mock.Anything, mock.Anything).
		Return(notifier.BatchResult{Failed: []string{"user@example.test"}})
	s.notifications.On("Delete", mock.Anything, int64(1)).Return(nil)
	s.publisher.On("ReleaseDead", mock.Anything, mock.Anything).Return(nil)

	s.Require().NoError(s.retrier.Releases(s.T().Context()))
	s.notifications.AssertNotCalled(s.T(), "IncrementRetry")
	s.publisher.AssertNotCalled(s.T(), "ReleaseSent")
}

func (s *RetrySuite) TestReleaseRetryIncrementsWhenAttemptsRemain() {
	s.notifications.On("ListRetryable", mock.Anything, maxRetries).
		Return([]domain.FailedNotification{s.failedNotification(0)}, nil)
	s.recipients.On("Recipient", mock.Anything, mock.Anything, mock.Anything).
		Return(domain.Recipient{Email: "user@example.test", UnsubToken: "t"}, nil)
	s.urls.On("UnsubscribeURL", mock.Anything).Return("https://example.test/u")
	s.relSender.On("SendReleaseNotifications", mock.Anything, mock.Anything).
		Return(notifier.BatchResult{Failed: []string{"user@example.test"}})
	s.notifications.On("IncrementRetry", mock.Anything, int64(1)).Return(nil)

	s.Require().NoError(s.retrier.Releases(s.T().Context()))
	s.notifications.AssertNotCalled(s.T(), "Delete")
	s.publisher.AssertNotCalled(s.T(), "ReleaseDead")
}

func (s *RetrySuite) TestReleaseRetryDropsUnsubscribedRecipient() {
	s.notifications.On("ListRetryable", mock.Anything, maxRetries).
		Return([]domain.FailedNotification{s.failedNotification(0)}, nil)
	s.recipients.On("Recipient", mock.Anything, mock.Anything, mock.Anything).
		Return(domain.Recipient{}, shareddomain.ErrNotFound)
	s.notifications.On("Delete", mock.Anything, int64(1)).Return(nil)

	s.Require().NoError(s.retrier.Releases(s.T().Context()))
	s.relSender.AssertNotCalled(s.T(), "SendReleaseNotifications")
	s.publisher.AssertNotCalled(s.T(), "ReleaseSent")
}

func (s *RetrySuite) TestConfirmationExpiredIsDeadLettered() {
	s.confirmations.On("ListExpired", mock.Anything, mock.Anything).
		Return([]domain.FailedConfirmation{s.failedConfirmation(0)}, nil)
	s.confirmations.On("ListRetryable", mock.Anything, maxRetries, mock.Anything).
		Return([]domain.FailedConfirmation{}, nil)
	s.confirmations.On("Delete", mock.Anything, int64(1)).Return(nil)
	s.publisher.On("ConfirmationDead", mock.Anything, mock.Anything).Return(nil)

	s.Require().NoError(s.retrier.Confirmations(s.T().Context()))
	s.confSender.AssertNotCalled(s.T(), "DeliverConfirmation")
}

func (s *RetrySuite) TestConfirmationRetrySucceeds() {
	s.confirmations.On("ListExpired", mock.Anything, mock.Anything).Return([]domain.FailedConfirmation{}, nil)
	s.confirmations.On("ListRetryable", mock.Anything, maxRetries, mock.Anything).
		Return([]domain.FailedConfirmation{s.failedConfirmation(0)}, nil)
	s.confSender.On("DeliverConfirmation", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	s.confirmations.On("Delete", mock.Anything, int64(1)).Return(nil)
	s.publisher.On("ConfirmationSent", mock.Anything, mock.Anything).Return(nil)

	s.Require().NoError(s.retrier.Confirmations(s.T().Context()))
}

func (s *RetrySuite) TestConfirmationRetryDeadLettersOnFinalAttempt() {
	s.confirmations.On("ListExpired", mock.Anything, mock.Anything).Return([]domain.FailedConfirmation{}, nil)
	s.confirmations.On("ListRetryable", mock.Anything, maxRetries, mock.Anything).
		Return([]domain.FailedConfirmation{s.failedConfirmation(maxRetries - 1)}, nil)
	s.confSender.On("DeliverConfirmation", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("smtp down"))
	s.confirmations.On("Delete", mock.Anything, int64(1)).Return(nil)
	s.publisher.On("ConfirmationDead", mock.Anything, mock.Anything).Return(nil)

	s.Require().NoError(s.retrier.Confirmations(s.T().Context()))
	s.confirmations.AssertNotCalled(s.T(), "IncrementRetry")
}

func (s *RetrySuite) TestConfirmationRetryIncrementsWhenAttemptsRemain() {
	s.confirmations.On("ListExpired", mock.Anything, mock.Anything).Return([]domain.FailedConfirmation{}, nil)
	s.confirmations.On("ListRetryable", mock.Anything, maxRetries, mock.Anything).
		Return([]domain.FailedConfirmation{s.failedConfirmation(0)}, nil)
	s.confSender.On("DeliverConfirmation", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("smtp down"))
	s.confirmations.On("IncrementRetry", mock.Anything, int64(1)).Return(nil)

	s.Require().NoError(s.retrier.Confirmations(s.T().Context()))
	s.confirmations.AssertNotCalled(s.T(), "Delete")
	s.publisher.AssertNotCalled(s.T(), "ConfirmationDead")
}

func (s *RetrySuite) TestReleaseRetryContinuesPastRecipientError() {
	fn1 := s.failedNotification(0)
	fn2 := s.failedNotification(0)
	fn2.ID = 2
	fn2.Email = "other@example.test"

	s.notifications.On("ListRetryable", mock.Anything, maxRetries).
		Return([]domain.FailedNotification{fn1, fn2}, nil)
	s.recipients.On("Recipient", mock.Anything, fn1.Email, mock.Anything).
		Return(domain.Recipient{}, shareddomain.ErrNotFound)
	s.recipients.On("Recipient", mock.Anything, fn2.Email, mock.Anything).
		Return(domain.Recipient{Email: fn2.Email, UnsubToken: "t"}, nil)
	s.notifications.On("Delete", mock.Anything, int64(1)).Return(nil)
	s.notifications.On("Delete", mock.Anything, int64(2)).Return(nil)
	s.urls.On("UnsubscribeURL", mock.Anything).Return("https://example.test/u")
	s.relSender.On("SendReleaseNotifications", mock.Anything, mock.Anything).Return(notifier.BatchResult{Sent: 1})
	s.publisher.On("ReleaseSent", mock.Anything, mock.Anything).Return(nil)

	s.Require().NoError(s.retrier.Releases(s.T().Context()))
}
