package scan

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/shared/entity"
)

type ScanSuite struct {
	suite.Suite
}

func TestScanSuite(t *testing.T) {
	suite.Run(t, new(ScanSuite))
}

const testRepoName = "owner/repo"

func testRepo(lastTag *string) *entity.Repository {
	return &entity.Repository{ID: uuid.New(), Name: testRepoName, LastSeenTag: lastTag}
}

func testSub(repoID uuid.UUID) *entity.Subscription {
	return &entity.Subscription{ID: uuid.New(), RepositoryID: repoID, Email: "user@example.com", Confirmed: true}
}

func observed(tag string) entity.ObservedRelease {
	return entity.ObservedRelease{
		Repo: testRepoName,
		Release: &entity.Release{
			TagName: tag,
			HTMLURL: "https://github.com/" + testRepoName + "/releases/tag/" + tag,
		},
	}
}

func (s *ScanSuite) run(repos *mockRepos, subs *mockSubs, scanner *mockScanner, n *mockNotifier) {
	uc := New(repos, subs, scanner, n, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := uc.Execute(s.T().Context(), Input{})
	s.Require().NoError(err)
}

func (s *ScanSuite) TestFirstSeen_SeedsSilently() {
	repo := testRepo(nil)

	repos := &mockRepos{}
	repos.On("GetAllWithSubscriptions", mock.Anything).Return([]*entity.Repository{repo}, nil)
	repos.On("UpdateLastSeenTag", mock.Anything, "owner/repo", "v1.0.0").Return(nil)
	defer repos.AssertExpectations(s.T())

	scanner := &mockScanner{}
	scanner.On("Scan", mock.Anything, []string{"owner/repo"}).
		Return([]entity.ObservedRelease{observed("v1.0.0")}, nil)
	defer scanner.AssertExpectations(s.T())

	subs := &mockSubs{}
	n := &mockNotifier{}

	s.run(repos, subs, scanner, n)

	subs.AssertNotCalled(s.T(), "GetConfirmedByRepoID", mock.Anything, mock.Anything)
	n.AssertNotCalled(s.T(), "Notify", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ScanSuite) TestTagUnchanged_NoNotification() {
	repo := testRepo(new("v1.0.0"))

	repos := &mockRepos{}
	repos.On("GetAllWithSubscriptions", mock.Anything).Return([]*entity.Repository{repo}, nil)
	defer repos.AssertExpectations(s.T())

	scanner := &mockScanner{}
	scanner.On("Scan", mock.Anything, mock.Anything).
		Return([]entity.ObservedRelease{observed("v1.0.0")}, nil)
	defer scanner.AssertExpectations(s.T())

	s.run(repos, &mockSubs{}, scanner, &mockNotifier{})

	repos.AssertNotCalled(s.T(), "UpdateLastSeenTag", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ScanSuite) TestNewRelease_NotifiesAndAdvances() {
	repo := testRepo(new("v1.0.0"))

	repos := &mockRepos{}
	repos.On("GetAllWithSubscriptions", mock.Anything).Return([]*entity.Repository{repo}, nil)
	repos.On("UpdateLastSeenTag", mock.Anything, "owner/repo", "v2.0.0").Return(nil)
	defer repos.AssertExpectations(s.T())

	scanner := &mockScanner{}
	scanner.On("Scan", mock.Anything, mock.Anything).
		Return([]entity.ObservedRelease{observed("v2.0.0")}, nil)
	defer scanner.AssertExpectations(s.T())

	subs := &mockSubs{}
	subs.On("GetConfirmedByRepoID", mock.Anything, repo.ID).Return([]*entity.Subscription{testSub(repo.ID)}, nil)
	defer subs.AssertExpectations(s.T())

	n := &mockNotifier{}
	n.On("Notify", mock.Anything, mock.Anything, repo,
		&entity.Release{TagName: "v2.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v2.0.0"},
	).Return(nil)
	defer n.AssertExpectations(s.T())

	s.run(repos, subs, scanner, n)
}

func (s *ScanSuite) TestSendFailure_TagNotAdvanced() {
	repo := testRepo(new("v1.0.0"))

	repos := &mockRepos{}
	repos.On("GetAllWithSubscriptions", mock.Anything).Return([]*entity.Repository{repo}, nil)
	defer repos.AssertExpectations(s.T())

	scanner := &mockScanner{}
	scanner.On("Scan", mock.Anything, mock.Anything).
		Return([]entity.ObservedRelease{observed("v2.0.0")}, nil)
	defer scanner.AssertExpectations(s.T())

	subs := &mockSubs{}
	subs.On("GetConfirmedByRepoID", mock.Anything, repo.ID).Return([]*entity.Subscription{testSub(repo.ID)}, nil)
	defer subs.AssertExpectations(s.T())

	n := &mockNotifier{}
	n.On("Notify", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("smtp error"))
	defer n.AssertExpectations(s.T())

	s.run(repos, subs, scanner, n)

	repos.AssertNotCalled(s.T(), "UpdateLastSeenTag", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ScanSuite) TestNoSubscribers_AdvancesTagOnly() {
	repo := testRepo(new("v1.0.0"))

	repos := &mockRepos{}
	repos.On("GetAllWithSubscriptions", mock.Anything).Return([]*entity.Repository{repo}, nil)
	repos.On("UpdateLastSeenTag", mock.Anything, "owner/repo", "v2.0.0").Return(nil)
	defer repos.AssertExpectations(s.T())

	scanner := &mockScanner{}
	scanner.On("Scan", mock.Anything, mock.Anything).
		Return([]entity.ObservedRelease{observed("v2.0.0")}, nil)
	defer scanner.AssertExpectations(s.T())

	subs := &mockSubs{}
	subs.On("GetConfirmedByRepoID", mock.Anything, repo.ID).Return([]*entity.Subscription{}, nil)
	defer subs.AssertExpectations(s.T())

	n := &mockNotifier{}

	s.run(repos, subs, scanner, n)

	n.AssertNotCalled(s.T(), "Notify", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ScanSuite) TestNoRepos_NoFetch() {
	repos := &mockRepos{}
	repos.On("GetAllWithSubscriptions", mock.Anything).Return([]*entity.Repository{}, nil)
	defer repos.AssertExpectations(s.T())

	scanner := &mockScanner{}

	s.run(repos, &mockSubs{}, scanner, &mockNotifier{})

	scanner.AssertNotCalled(s.T(), "Scan", mock.Anything, mock.Anything)
}

func (s *ScanSuite) TestListReposError_ReturnsError() {
	repos := &mockRepos{}
	repos.On("GetAllWithSubscriptions", mock.Anything).Return(nil, errors.New("db error"))
	defer repos.AssertExpectations(s.T())

	uc := New(repos, &mockSubs{}, &mockScanner{}, &mockNotifier{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := uc.Execute(s.T().Context(), Input{})
	s.Error(err)
}

func (s *ScanSuite) TestFetchError_ReturnsError() {
	repo := testRepo(new("v1.0.0"))

	repos := &mockRepos{}
	repos.On("GetAllWithSubscriptions", mock.Anything).Return([]*entity.Repository{repo}, nil)
	defer repos.AssertExpectations(s.T())

	scanner := &mockScanner{}
	scanner.On("Scan", mock.Anything, mock.Anything).Return(nil, errors.New("rpc error"))
	defer scanner.AssertExpectations(s.T())

	uc := New(repos, &mockSubs{}, scanner, &mockNotifier{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := uc.Execute(s.T().Context(), Input{})
	s.Error(err)
}
