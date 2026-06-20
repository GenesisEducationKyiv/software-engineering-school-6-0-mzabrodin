package advancetag

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type AdvanceTagSuite struct {
	suite.Suite

	repo *mockRepository
	uc   *UseCase
}

func TestAdvanceTagSuite(t *testing.T) {
	suite.Run(t, new(AdvanceTagSuite))
}

func (s *AdvanceTagSuite) SetupTest() {
	s.repo = &mockRepository{}
	s.uc = New(s.repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (s *AdvanceTagSuite) TearDownTest() {
	s.repo.AssertExpectations(s.T())
}

func (s *AdvanceTagSuite) TestAdvancesWhenSent() {
	s.repo.On("AdvanceTag", mock.Anything, "golang/go", "v1.2.0").Return(nil)

	err := s.uc.Execute(s.T().Context(), Input{RepoName: "golang/go", Tag: "v1.2.0", SentCount: 3})

	s.Require().NoError(err)
}

func (s *AdvanceTagSuite) TestDoesNotAdvanceWhenNoneSent() {
	err := s.uc.Execute(s.T().Context(), Input{RepoName: "golang/go", Tag: "v1.2.0", SentCount: 0})

	s.Require().NoError(err)
	s.repo.AssertNotCalled(s.T(), "AdvanceTag")
}

func (s *AdvanceTagSuite) TestDoesNotAdvanceWhenNegative() {
	err := s.uc.Execute(s.T().Context(), Input{RepoName: "golang/go", Tag: "v1.2.0", SentCount: -1})

	s.Require().NoError(err)
	s.repo.AssertNotCalled(s.T(), "AdvanceTag")
}
