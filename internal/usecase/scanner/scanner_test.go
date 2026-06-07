package scanner

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/entity"
)

func testRepo(name string, lastTag *string) *entity.Repository {
	return &entity.Repository{ID: uuid.New(), Name: name, LastSeenTag: lastTag}
}

func testSub(repoID uuid.UUID) *entity.Subscription {
	return &entity.Subscription{
		ID:               uuid.New(),
		RepositoryID:     repoID,
		Email:            "user@example.com",
		UnsubscribeToken: "tok",
	}
}

func testSubRepo(repoID uuid.UUID) *mockSubRepository {
	m := &mockSubRepository{}
	m.On("GetConfirmedByRepoID", mock.Anything, repoID).
		Return([]*entity.Subscription{testSub(repoID)}, nil)
	return m
}

func newScanner(repos *mockRepoRepository, subs *mockSubRepository, gh *mockGitHub, n *mockNotifier) *Scanner {
	return NewScanner(repos, subs, gh, n, 2, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

type ScannerSuite struct {
	suite.Suite
}

func TestScannerSuite(t *testing.T) {
	suite.Run(t, new(ScannerSuite))
}

func (s *ScannerSuite) TestCheckRepo_NoRelease_Skipped() {
	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo").Return(nil, entity.ErrNoRelease)
	defer gh.AssertExpectations(s.T())

	sc := newScanner(&mockRepoRepository{}, &mockSubRepository{}, gh, &mockNotifier{})
	s.NoError(sc.checkRepo(s.T().Context(), testRepo("owner/repo", nil)))
}

func (s *ScannerSuite) TestCheckRepo_GlobalGitHubError_ReturnsError() {
	cases := []struct {
		name string
		err  error
	}{
		{"rate limited (wrapped)", fmt.Errorf("%w, retry after 60s", entity.ErrRateLimited)},
		{"unauthorized", entity.ErrUnauthorized},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			gh := &mockGitHub{}
			gh.On("GetLatestRelease", mock.Anything, "owner", "repo").Return(nil, tc.err)
			defer gh.AssertExpectations(s.T())

			sc := newScanner(&mockRepoRepository{}, &mockSubRepository{}, gh, &mockNotifier{})
			s.Error(sc.checkRepo(s.T().Context(), testRepo("owner/repo", nil)))
		})
	}
}

func (s *ScannerSuite) TestCheckRepo_TagUnchanged_NoNotification() {
	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo").
		Return(&entity.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"}, nil)
	defer gh.AssertExpectations(s.T())

	sc := newScanner(&mockRepoRepository{}, &mockSubRepository{}, gh, &mockNotifier{})
	s.Require().NoError(sc.checkRepo(s.T().Context(), testRepo("owner/repo", new("v1.0.0"))))
}

func (s *ScannerSuite) TestCheckRepo_NewRelease_SendsNotificationAndUpdatesTag() {
	cases := []struct {
		name       string
		lastTag    *string
		releaseTag string
	}{
		{"tag changed", new("v1.0.0"), "v2.0.0"},
		{"first release", nil, "v1.0.0"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			repo := testRepo("owner/repo", tc.lastTag)

			gh := &mockGitHub{}
			gh.On("GetLatestRelease", mock.Anything, "owner", "repo").
				Return(&entity.Release{TagName: tc.releaseTag, HTMLURL: "https://github.com/owner/repo/releases/tag/" + tc.releaseTag}, nil)
			defer gh.AssertExpectations(s.T())

			subs := testSubRepo(repo.ID)
			defer subs.AssertExpectations(s.T())

			repos := &mockRepoRepository{}
			repos.On("UpdateLastSeenTag", mock.Anything, "owner/repo", tc.releaseTag).Return(nil)
			defer repos.AssertExpectations(s.T())

			n := &mockNotifier{}
			var subCount int
			n.On("Notify", mock.Anything, mock.Anything, repo, mock.Anything).
				Run(func(args mock.Arguments) {
					v, _ := args.Get(1).([]*entity.Subscription)
					subCount = len(v)
				}).Return(nil)
			defer n.AssertExpectations(s.T())

			sc := newScanner(repos, subs, gh, n)
			s.Require().NoError(sc.checkRepo(s.T().Context(), repo))
			s.Equal(1, subCount)
		})
	}
}

func (s *ScannerSuite) TestCheckRepo_MailerError_TagNotUpdated() {
	repo := testRepo("owner/repo", nil)

	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo").
		Return(&entity.Release{TagName: "v2.0.0", HTMLURL: "..."}, nil)
	defer gh.AssertExpectations(s.T())

	subs := testSubRepo(repo.ID)
	defer subs.AssertExpectations(s.T())

	n := &mockNotifier{}
	n.On("Notify", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("smtp error"))
	defer n.AssertExpectations(s.T())

	sc := newScanner(&mockRepoRepository{}, subs, gh, n)
	s.Error(sc.checkRepo(s.T().Context(), repo))
}

func (s *ScannerSuite) TestCheckRepo_NoSubscribers_UpdatesTagOnly() {
	newTag := "v2.0.0"

	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo").
		Return(&entity.Release{TagName: newTag, HTMLURL: "..."}, nil)
	defer gh.AssertExpectations(s.T())

	subs := &mockSubRepository{}
	subs.On("GetConfirmedByRepoID", mock.Anything, mock.Anything).
		Return([]*entity.Subscription{}, nil)
	defer subs.AssertExpectations(s.T())

	repos := &mockRepoRepository{}
	repos.On("UpdateLastSeenTag", mock.Anything, "owner/repo", newTag).Return(nil)
	defer repos.AssertExpectations(s.T())

	sc := newScanner(repos, subs, gh, &mockNotifier{})
	s.Require().NoError(sc.checkRepo(s.T().Context(), testRepo("owner/repo", nil)))
}

func (s *ScannerSuite) TestCheckRepo_InvalidRepo_ReturnsError() {
	sc := newScanner(&mockRepoRepository{}, &mockSubRepository{}, &mockGitHub{}, &mockNotifier{})
	s.Error(sc.checkRepo(s.T().Context(), testRepo("notavalidrepo", nil)))
}

func (s *ScannerSuite) TestCheckRepo_SubRepoError_ReturnsError() {
	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo").
		Return(&entity.Release{TagName: "v2.0.0", HTMLURL: "..."}, nil)
	defer gh.AssertExpectations(s.T())

	subs := &mockSubRepository{}
	subs.On("GetConfirmedByRepoID", mock.Anything, mock.Anything).
		Return(nil, errors.New("db error"))
	defer subs.AssertExpectations(s.T())

	sc := newScanner(&mockRepoRepository{}, subs, gh, &mockNotifier{})
	s.Error(sc.checkRepo(s.T().Context(), testRepo("owner/repo", nil)))
}

func (s *ScannerSuite) TestCheckRepo_UpdateTagError_NoSubs_ReturnsError() {
	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo").
		Return(&entity.Release{TagName: "v2.0.0", HTMLURL: "..."}, nil)
	defer gh.AssertExpectations(s.T())

	subs := &mockSubRepository{}
	subs.On("GetConfirmedByRepoID", mock.Anything, mock.Anything).
		Return([]*entity.Subscription{}, nil)
	defer subs.AssertExpectations(s.T())

	repos := &mockRepoRepository{}
	repos.On("UpdateLastSeenTag", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("db error"))
	defer repos.AssertExpectations(s.T())

	sc := newScanner(repos, subs, gh, &mockNotifier{})
	s.Error(sc.checkRepo(s.T().Context(), testRepo("owner/repo", nil)))
}

func (s *ScannerSuite) TestCheckRepo_UpdateTagError_WithSubs_ReturnsError() {
	repo := testRepo("owner/repo", nil)

	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo").
		Return(&entity.Release{TagName: "v2.0.0", HTMLURL: "..."}, nil)
	defer gh.AssertExpectations(s.T())

	subs := testSubRepo(repo.ID)
	defer subs.AssertExpectations(s.T())

	n := &mockNotifier{}
	n.On("Notify", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	defer n.AssertExpectations(s.T())

	repos := &mockRepoRepository{}
	repos.On("UpdateLastSeenTag", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("db error"))
	defer repos.AssertExpectations(s.T())

	sc := newScanner(repos, subs, gh, n)
	s.Error(sc.checkRepo(s.T().Context(), repo))
}

func (s *ScannerSuite) TestRun_FetchReposError_ReturnsError() {
	repos := &mockRepoRepository{}
	repos.On("GetAllWithSubscriptions", mock.Anything).Return(nil, errors.New("db error"))
	defer repos.AssertExpectations(s.T())

	sc := newScanner(repos, &mockSubRepository{}, &mockGitHub{}, &mockNotifier{})
	s.Error(sc.Run(s.T().Context()))
}

func (s *ScannerSuite) TestRun_AllReposProcessed() {
	repoList := []*entity.Repository{
		testRepo("owner/repo1", nil),
		testRepo("owner/repo2", nil),
		testRepo("owner/repo3", nil),
	}

	repos := &mockRepoRepository{}
	repos.On("GetAllWithSubscriptions", mock.Anything).Return(repoList, nil)
	defer repos.AssertExpectations(s.T())

	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo1").Return(nil, entity.ErrNoRelease)
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo2").Return(nil, entity.ErrNoRelease)
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo3").Return(nil, entity.ErrNoRelease)
	defer gh.AssertExpectations(s.T())

	sc := newScanner(repos, &mockSubRepository{}, gh, &mockNotifier{})
	s.NoError(sc.Run(s.T().Context()))
}

func (s *ScannerSuite) TestRun_PerRepoErrorIsolated() {
	repoList := []*entity.Repository{
		testRepo("owner/repo1", nil),
		testRepo("owner/repo2", nil),
	}

	repos := &mockRepoRepository{}
	repos.On("GetAllWithSubscriptions", mock.Anything).Return(repoList, nil)
	defer repos.AssertExpectations(s.T())

	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo1").Return(nil, errors.New("boom"))
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo2").Return(nil, entity.ErrNoRelease)
	defer gh.AssertExpectations(s.T())

	sc := newScanner(repos, &mockSubRepository{}, gh, &mockNotifier{})
	s.NoError(sc.Run(s.T().Context()))
}

func (s *ScannerSuite) TestRun_RateLimited_StopsScan() {
	repoList := []*entity.Repository{
		testRepo("owner/repo1", nil),
		testRepo("owner/repo2", nil),
		testRepo("owner/repo3", nil),
	}

	repos := &mockRepoRepository{}
	repos.On("GetAllWithSubscriptions", mock.Anything).Return(repoList, nil)
	defer repos.AssertExpectations(s.T())

	// Only repo1 is expected to reach GitHub; the rate limit must stop the pass
	// before repo2/repo3 are scanned. A single worker keeps the order deterministic.
	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo1").Return(nil, entity.ErrRateLimited)
	defer gh.AssertExpectations(s.T())

	sc := NewScanner(
		repos,
		&mockSubRepository{},
		gh,
		&mockNotifier{},
		1,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	s.NoError(sc.Run(s.T().Context()))
}
