package compensate

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type mockSubs struct{ mock.Mock }

func (m *mockSubs) DeletePendingByEmailAndRepo(ctx context.Context, email, repoName string) (bool, error) {
	args := m.Called(ctx, email, repoName)

	return args.Bool(0), args.Error(1)
}

type CompensateSuite struct {
	suite.Suite

	subs *mockSubs
	uc   *UseCase
}

func TestCompensateSuite(t *testing.T) {
	suite.Run(t, new(CompensateSuite))
}

func (s *CompensateSuite) SetupTest() {
	s.subs = &mockSubs{}
	s.uc = New(s.subs, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (s *CompensateSuite) TearDownTest() {
	s.subs.AssertExpectations(s.T())
}

func (s *CompensateSuite) TestDeletesPending() {
	s.subs.On("DeletePendingByEmailAndRepo", mock.Anything, "user@example.test", "golang/go").Return(true, nil)

	deleted, err := s.uc.Execute(s.T().Context(), Input{Email: "user@example.test", RepoName: "golang/go"})
	s.Require().NoError(err)
	s.True(deleted)
}

func (s *CompensateSuite) TestNoPendingRowIsNotAnError() {
	s.subs.On("DeletePendingByEmailAndRepo", mock.Anything, "user@example.test", "golang/go").Return(false, nil)

	deleted, err := s.uc.Execute(s.T().Context(), Input{Email: "user@example.test", RepoName: "golang/go"})
	s.Require().NoError(err)
	s.False(deleted)
}

func (s *CompensateSuite) TestPropagatesError() {
	s.subs.On("DeletePendingByEmailAndRepo", mock.Anything, "user@example.test", "golang/go").
		Return(false, errors.New("db down"))

	_, err := s.uc.Execute(s.T().Context(), Input{Email: "user@example.test", RepoName: "golang/go"})
	s.Require().Error(err)
}
