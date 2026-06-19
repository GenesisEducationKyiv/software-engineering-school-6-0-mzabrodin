package readmodel

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/shared/events"
)

type ProjectorSuite struct {
	suite.Suite

	store     *mockStore
	projector *Projector
}

func TestProjectorSuite(t *testing.T) {
	suite.Run(t, new(ProjectorSuite))
}

func (s *ProjectorSuite) SetupTest() {
	s.store = &mockStore{}
	s.projector = New(s.store)
}

func (s *ProjectorSuite) TearDownTest() {
	s.store.AssertExpectations(s.T())
}

func (s *ProjectorSuite) TestConfirmedUpserts() {
	s.store.On("Upsert", mock.Anything, "user@example.test", "golang/go", "tok").Return(nil)

	err := s.projector.Confirmed(s.T().Context(), events.SubscriptionConfirmed{
		SagaID:     uuid.NewString(),
		Email:      "user@example.test",
		RepoName:   "golang/go",
		UnsubToken: "tok",
	})

	s.Require().NoError(err)
}

func (s *ProjectorSuite) TestRemovedDeletes() {
	s.store.On("Delete", mock.Anything, "user@example.test", "golang/go").Return(nil)

	err := s.projector.Removed(s.T().Context(), events.SubscriptionRemoved{
		SagaID:   uuid.NewString(),
		Email:    "user@example.test",
		RepoName: "golang/go",
	})

	s.Require().NoError(err)
}
