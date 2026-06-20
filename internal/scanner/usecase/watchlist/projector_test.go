package watchlist

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/shared/events"
)

type ProjectorSuite struct {
	suite.Suite

	repo      *mockRepository
	projector *Projector
}

func TestProjectorSuite(t *testing.T) {
	suite.Run(t, new(ProjectorSuite))
}

func (s *ProjectorSuite) SetupTest() {
	s.repo = &mockRepository{}
	s.projector = New(s.repo)
}

func (s *ProjectorSuite) TearDownTest() {
	s.repo.AssertExpectations(s.T())
}

func (s *ProjectorSuite) TestConfirmedIncrements() {
	s.repo.On("IncrementSubscriber", mock.Anything, "golang/go").Return(nil)

	err := s.projector.Confirmed(s.T().Context(), events.SubscriptionConfirmed{
		SagaID:     uuid.NewString(),
		Email:      "user@example.test",
		RepoName:   "golang/go",
		UnsubToken: "tok",
	})

	s.Require().NoError(err)
}

func (s *ProjectorSuite) TestRemovedDecrements() {
	s.repo.On("DecrementSubscriber", mock.Anything, "golang/go").Return(nil)

	err := s.projector.Removed(s.T().Context(), events.SubscriptionRemoved{
		SagaID:   uuid.NewString(),
		Email:    "user@example.test",
		RepoName: "golang/go",
	})

	s.Require().NoError(err)
}
