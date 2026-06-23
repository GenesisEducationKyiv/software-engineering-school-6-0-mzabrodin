package coordinator

import (
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/saga/domain"
	"github-release-notifier/internal/shared/events"
)

type CoordinatorSuite struct {
	suite.Suite

	repo  *mockRepo
	pub   *mockPublisher
	tx    *mockTransactor
	coord *Coordinator
}

func TestCoordinatorSuite(t *testing.T) {
	suite.Run(t, new(CoordinatorSuite))
}

func (s *CoordinatorSuite) SetupTest() {
	s.repo = &mockRepo{}
	s.pub = &mockPublisher{}
	s.tx = &mockTransactor{}
	s.coord = New(
		s.repo,
		NewNATSCompensator(s.repo, s.pub, s.tx),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func (s *CoordinatorSuite) TearDownTest() {
	s.repo.AssertExpectations(s.T())
	s.pub.AssertExpectations(s.T())
	s.tx.AssertExpectations(s.T())
}

func (s *CoordinatorSuite) TestOnPendingStartsSaga() {
	id := uuid.New()

	s.repo.On("Start", mock.Anything, domain.Saga{
		ID:       id,
		Type:     domain.SagaTypeSubscribe,
		Email:    "user@example.test",
		RepoName: "golang/go",
	}).Return(nil)

	s.Require().NoError(s.coord.OnPending(s.T().Context(), id.String(), "user@example.test", "golang/go"))
}

func (s *CoordinatorSuite) TestOnConfirmationSentAdvances() {
	id := uuid.New()
	s.repo.On("TransitionByID", mock.Anything, id, domain.StateConfirmationSent).Return(true, nil)

	s.Require().NoError(s.coord.OnConfirmationSent(s.T().Context(), id.String()))
}

func (s *CoordinatorSuite) TestOnConfirmedCompletesByEmailRepo() {
	s.repo.On("TransitionByEmailRepo", mock.Anything,
		domain.SagaTypeSubscribe, "user@example.test", "golang/go", domain.StateCompleted).Return(true, nil)

	s.Require().NoError(s.coord.OnConfirmed(s.T().Context(), "user@example.test", "golang/go"))
}

func (s *CoordinatorSuite) TestOnExpiredByEmailRepo() {
	s.repo.On("TransitionByEmailRepo", mock.Anything,
		domain.SagaTypeSubscribe, "user@example.test", "golang/go", domain.StateExpired).Return(true, nil)

	s.Require().NoError(s.coord.OnExpired(s.T().Context(), "user@example.test", "golang/go"))
}

func (s *CoordinatorSuite) TestOnConfirmationDeadCompensatesFromPending() {
	id := uuid.New()
	saga := domain.Saga{
		ID: id, Type: domain.SagaTypeSubscribe, State: domain.StatePending,
		Email: "user@example.test", RepoName: "golang/go",
	}

	s.repo.On("Get", mock.Anything, id).Return(saga, true, nil)
	s.tx.On("Within", mock.Anything).Return(nil)
	s.repo.On("TransitionByID", mock.Anything, id, domain.StateCompensated).Return(true, nil)
	s.pub.On("Compensate", mock.Anything, events.SagaCompensate{
		SagaID:   id.String(),
		SagaType: string(domain.SagaTypeSubscribe),
		Email:    "user@example.test",
		RepoName: "golang/go",
	}).Return(nil)
	s.pub.On("Notify").Return()

	s.Require().NoError(s.coord.OnConfirmationDead(s.T().Context(), id.String()))
}

func (s *CoordinatorSuite) TestOnConfirmationDeadAfterCompletedIsNoOp() {
	id := uuid.New()
	saga := domain.Saga{ID: id, Type: domain.SagaTypeSubscribe, State: domain.StateCompleted}

	s.repo.On("Get", mock.Anything, id).Return(saga, true, nil)

	s.Require().NoError(s.coord.OnConfirmationDead(s.T().Context(), id.String()))
	s.pub.AssertNotCalled(s.T(), "Compensate", mock.Anything, mock.Anything)
}

func (s *CoordinatorSuite) TestOnConfirmationDeadUnknownSagaIsNoOp() {
	id := uuid.New()
	s.repo.On("Get", mock.Anything, id).Return(domain.Saga{}, false, nil)

	s.Require().NoError(s.coord.OnConfirmationDead(s.T().Context(), id.String()))
	s.pub.AssertNotCalled(s.T(), "Compensate", mock.Anything, mock.Anything)
}
