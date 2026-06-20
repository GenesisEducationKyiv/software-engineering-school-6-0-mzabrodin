package watch

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/scanner/domain"
	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/events"
)

type WatchSuite struct {
	suite.Suite

	repos     *mockRepository
	scanner   *mockScanner
	publisher *mockPublisher
	uc        *UseCase
}

func TestWatchSuite(t *testing.T) {
	suite.Run(t, new(WatchSuite))
}

func (s *WatchSuite) SetupTest() {
	s.repos = &mockRepository{}
	s.scanner = &mockScanner{}
	s.publisher = &mockPublisher{}
	s.uc = New(s.repos, s.scanner, s.publisher, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (s *WatchSuite) TearDownTest() {
	s.repos.AssertExpectations(s.T())
	s.scanner.AssertExpectations(s.T())
	s.publisher.AssertExpectations(s.T())
}

func ptr(s string) *string { return &s }

func observed(repo, tag string) entity.ObservedRelease {
	return entity.ObservedRelease{
		Repo:    repo,
		Release: &entity.Release{TagName: tag, HTMLURL: "https://example.test/" + tag},
	}
}

func detectedFor(repo, tag string) any {
	return mock.MatchedBy(func(ev events.ReleaseDetected) bool {
		return ev.RepoName == repo && ev.Tag == tag && ev.ReleaseURL == "https://example.test/"+tag && ev.SagaID != ""
	})
}

func (s *WatchSuite) TestSeedsSilentlyWhenNoTag() {
	s.repos.On("ListWatched", mock.Anything).
		Return([]domain.WatchedRepo{{RepoName: "golang/go", LastSeenTag: nil}}, nil)
	s.scanner.On("Scan", mock.Anything, []string{"golang/go"}).
		Return([]entity.ObservedRelease{observed("golang/go", "v1.0.0")}, nil)
	s.repos.On("AdvanceTag", mock.Anything, "golang/go", "v1.0.0").Return(nil)

	s.Require().NoError(s.uc.Run(s.T().Context()))
	s.publisher.AssertNotCalled(s.T(), "ReleaseDetected")
}

func (s *WatchSuite) TestPublishesOnNewTag() {
	s.repos.On("ListWatched", mock.Anything).
		Return([]domain.WatchedRepo{{RepoName: "golang/go", LastSeenTag: ptr("v1.0.0")}}, nil)
	s.scanner.On("Scan", mock.Anything, []string{"golang/go"}).
		Return([]entity.ObservedRelease{observed("golang/go", "v1.1.0")}, nil)
	s.publisher.On("ReleaseDetected", mock.Anything, detectedFor("golang/go", "v1.1.0")).Return(nil)

	s.Require().NoError(s.uc.Run(s.T().Context()))
	s.repos.AssertNotCalled(s.T(), "AdvanceTag")
}

func (s *WatchSuite) TestSkipsWhenTagUnchanged() {
	s.repos.On("ListWatched", mock.Anything).
		Return([]domain.WatchedRepo{{RepoName: "golang/go", LastSeenTag: ptr("v1.0.0")}}, nil)
	s.scanner.On("Scan", mock.Anything, []string{"golang/go"}).
		Return([]entity.ObservedRelease{observed("golang/go", "v1.0.0")}, nil)

	s.Require().NoError(s.uc.Run(s.T().Context()))
	s.repos.AssertNotCalled(s.T(), "AdvanceTag")
	s.publisher.AssertNotCalled(s.T(), "ReleaseDetected")
}

func (s *WatchSuite) TestContinuesWhenPublishFails() {
	s.repos.On("ListWatched", mock.Anything).Return([]domain.WatchedRepo{
		{RepoName: "golang/go", LastSeenTag: ptr("v1.0.0")},
		{RepoName: "rust-lang/rust", LastSeenTag: ptr("v2.0.0")},
	}, nil)
	s.scanner.On("Scan", mock.Anything, []string{"golang/go", "rust-lang/rust"}).Return([]entity.ObservedRelease{
		observed("golang/go", "v1.1.0"),
		observed("rust-lang/rust", "v2.1.0"),
	}, nil)
	s.publisher.On("ReleaseDetected", mock.Anything, detectedFor("golang/go", "v1.1.0")).
		Return(errors.New("nats down"))
	s.publisher.On("ReleaseDetected", mock.Anything, detectedFor("rust-lang/rust", "v2.1.0")).Return(nil)

	s.Require().NoError(s.uc.Run(s.T().Context()))
	s.repos.AssertNotCalled(s.T(), "AdvanceTag")
	s.publisher.AssertNumberOfCalls(s.T(), "ReleaseDetected", 2)
}

func (s *WatchSuite) TestNoWatchedReposReturnsEarly() {
	s.repos.On("ListWatched", mock.Anything).Return([]domain.WatchedRepo{}, nil)

	s.Require().NoError(s.uc.Run(s.T().Context()))
	s.scanner.AssertNotCalled(s.T(), "Scan")
}
