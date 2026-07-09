package coordinator

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/saga/domain"
)

type mockCompensationClient struct{ mock.Mock }

func (m *mockCompensationClient) Compensate(
	ctx context.Context,
	sagaID, sagaType, email, repoName string,
) (bool, error) {
	args := m.Called(ctx, sagaID, sagaType, email, repoName)

	return args.Bool(0), args.Error(1)
}

type GRPCCompensatorSuite struct {
	suite.Suite

	repo   *mockRepo
	client *mockCompensationClient
	comp   *GRPCCompensator
	saga   domain.Saga
}

func TestGRPCCompensatorSuite(t *testing.T) {
	suite.Run(t, new(GRPCCompensatorSuite))
}

func (s *GRPCCompensatorSuite) SetupTest() {
	s.repo = &mockRepo{}
	s.client = &mockCompensationClient{}
	s.comp = NewGRPCCompensator(s.repo, s.client)
	s.saga = domain.Saga{
		ID: uuid.New(), Type: domain.SagaTypeSubscribe,
		State: domain.StatePending, Email: "user@example.test", RepoName: "golang/go",
	}
}

func (s *GRPCCompensatorSuite) TearDownTest() {
	s.repo.AssertExpectations(s.T())
	s.client.AssertExpectations(s.T())
}

func (s *GRPCCompensatorSuite) TestTransitionsThenCallsRPC() {
	s.repo.On("TransitionByID", mock.Anything, s.saga.ID, domain.StateCompensated).Return(true, nil)
	s.client.On("Compensate", mock.Anything,
		s.saga.ID.String(), string(domain.SagaTypeSubscribe), "user@example.test", "golang/go").
		Return(true, nil)

	s.Require().NoError(s.comp.Compensate(s.T().Context(), s.saga))
}

func (s *GRPCCompensatorSuite) TestRPCErrorPropagates() {
	s.repo.On("TransitionByID", mock.Anything, s.saga.ID, domain.StateCompensated).Return(true, nil)
	s.client.On("Compensate", mock.Anything,
		s.saga.ID.String(), string(domain.SagaTypeSubscribe), "user@example.test", "golang/go").
		Return(false, errors.New("unavailable"))

	s.Require().Error(s.comp.Compensate(s.T().Context(), s.saga))
}

func (s *GRPCCompensatorSuite) TestTransitionErrorSkipsRPC() {
	s.repo.On("TransitionByID", mock.Anything, s.saga.ID, domain.StateCompensated).
		Return(false, errors.New("db down"))

	s.Require().Error(s.comp.Compensate(s.T().Context(), s.saga))
	s.client.AssertNotCalled(
		s.T(),
		"Compensate",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	)
}
