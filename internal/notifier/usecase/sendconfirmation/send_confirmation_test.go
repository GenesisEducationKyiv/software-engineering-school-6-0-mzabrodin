package sendconfirmation

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/notifier/domain"
	"github-release-notifier/internal/shared/events"
)

type SendConfirmationSuite struct {
	suite.Suite

	sender    *mockSender
	failed    *mockFailedStore
	publisher *mockPublisher
	uc        *UseCase
	input     Input
}

func TestSendConfirmationSuite(t *testing.T) {
	suite.Run(t, new(SendConfirmationSuite))
}

func (s *SendConfirmationSuite) SetupTest() {
	s.sender = &mockSender{}
	s.failed = &mockFailedStore{}
	s.publisher = &mockPublisher{}
	s.publisher.On("Notify").Return().Maybe()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s.uc = New(s.sender, s.failed, mockTransactor{}, s.publisher, log)

	s.input = Input{
		SagaID:     uuid.NewString(),
		Email:      "user@example.test",
		RepoName:   "golang/go",
		ConfirmURL: "https://example.test/api/confirm/token",
	}
}

func (s *SendConfirmationSuite) TearDownTest() {
	s.sender.AssertExpectations(s.T())
	s.failed.AssertExpectations(s.T())
	s.publisher.AssertExpectations(s.T())
}

func (s *SendConfirmationSuite) TestSendsAndPublishesSent() {
	s.sender.On("DeliverConfirmation", mock.Anything, s.input.Email, s.input.RepoName, s.input.ConfirmURL).Return(nil)

	var sent events.NotificationConfirmationSent
	s.publisher.On("ConfirmationSent", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { sent, _ = args.Get(1).(events.NotificationConfirmationSent) }).
		Return(nil)

	err := s.uc.Execute(s.T().Context(), s.input)

	s.Require().NoError(err)
	s.Equal(s.input.Email, sent.Email)
	s.failed.AssertNotCalled(s.T(), "Add")
}

func (s *SendConfirmationSuite) TestRecordsFailureAndPublishesFailed() {
	s.sender.On("DeliverConfirmation", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("smtp down"))

	var recorded *domain.FailedConfirmation
	s.failed.On("Add", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { recorded, _ = args.Get(1).(*domain.FailedConfirmation) }).
		Return(nil)
	s.publisher.On("ConfirmationFailed", mock.Anything, mock.Anything).Return(nil)

	err := s.uc.Execute(s.T().Context(), s.input)

	s.Require().NoError(err)
	s.Require().NotNil(recorded)
	s.Equal(s.input.Email, recorded.Email)
	s.Equal(s.input.ConfirmURL, recorded.ConfirmURL)
	s.publisher.AssertNotCalled(s.T(), "ConfirmationSent")
}

func (s *SendConfirmationSuite) TestReturnsErrorWhenRecordFails() {
	s.sender.On("DeliverConfirmation", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("smtp down"))
	s.failed.On("Add", mock.Anything, mock.Anything).Return(errors.New("db down"))

	err := s.uc.Execute(s.T().Context(), s.input)

	s.Require().Error(err)
}

func (s *SendConfirmationSuite) TestReturnsErrorWhenEnqueueFails() {
	s.sender.On("DeliverConfirmation", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("smtp down"))
	s.failed.On("Add", mock.Anything, mock.Anything).Return(nil)
	s.publisher.On("ConfirmationFailed", mock.Anything, mock.Anything).Return(errors.New("db down"))

	err := s.uc.Execute(s.T().Context(), s.input)

	// The outbox enqueue is part of the tx; if it fails the whole handler fails so the
	// message is redelivered rather than the failure event being silently lost.
	s.Require().Error(err)
}
