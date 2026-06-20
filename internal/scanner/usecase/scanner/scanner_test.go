package scanner

import (
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/scanner/domain"
	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/github"
)

func release(tag string) *entity.Release {
	return &entity.Release{TagName: tag, HTMLURL: "https://github.com/owner/repo/releases/tag/" + tag}
}

func newScanner(gh *mockGitHub, workers int) *Scanner {
	return New(gh, workers, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func repoNames(observed []domain.ObservedRelease) map[string]struct{} {
	out := make(map[string]struct{}, len(observed))
	for _, o := range observed {
		out[o.Repo] = struct{}{}
	}
	return out
}

type ScannerSuite struct {
	suite.Suite
}

func TestScannerSuite(t *testing.T) {
	suite.Run(t, new(ScannerSuite))
}

func (s *ScannerSuite) TestFetchesReleases() {
	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo1").Return(release("v1.0.0"), nil)
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo2").Return(release("v2.0.0"), nil)
	defer gh.AssertExpectations(s.T())

	observed, err := newScanner(gh, 2).Scan(s.T().Context(), []string{"owner/repo1", "owner/repo2"})
	s.Require().NoError(err)
	s.Len(observed, 2)
	s.Equal(map[string]struct{}{"owner/repo1": {}, "owner/repo2": {}}, repoNames(observed))
}

func (s *ScannerSuite) TestNoRelease_Omitted() {
	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo1").Return(release("v1.0.0"), nil)
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo2").Return(nil, github.ErrNoRelease)
	defer gh.AssertExpectations(s.T())

	observed, err := newScanner(gh, 2).Scan(s.T().Context(), []string{"owner/repo1", "owner/repo2"})
	s.Require().NoError(err)
	s.Equal(map[string]struct{}{"owner/repo1": {}}, repoNames(observed))
}

func (s *ScannerSuite) TestPerRepoErrorIsolated() {
	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo1").Return(nil, fmt.Errorf("boom"))
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo2").Return(release("v2.0.0"), nil)
	defer gh.AssertExpectations(s.T())

	observed, err := newScanner(gh, 2).Scan(s.T().Context(), []string{"owner/repo1", "owner/repo2"})
	s.Require().NoError(err)
	s.Equal(map[string]struct{}{"owner/repo2": {}}, repoNames(observed))
}

func (s *ScannerSuite) TestInvalidRepoName_Skipped() {
	observed, err := newScanner(&mockGitHub{}, 2).Scan(s.T().Context(), []string{"notavalidrepo"})
	s.Require().NoError(err)
	s.Empty(observed)
}

func (s *ScannerSuite) TestRateLimited_StopsPass() {
	cases := []struct {
		name string
		err  error
	}{
		{"rate limited (wrapped)", fmt.Errorf("%w, retry after 60s", github.ErrRateLimited)},
		{"unauthorized", github.ErrUnauthorized},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			gh := &mockGitHub{}
			gh.On("GetLatestRelease", mock.Anything, "owner", "repo1").Return(nil, tc.err)
			defer gh.AssertExpectations(s.T())

			observed, err := newScanner(gh, 1).
				Scan(s.T().Context(), []string{"owner/repo1", "owner/repo2", "owner/repo3"})
			s.Require().NoError(err)
			s.Empty(observed)
		})
	}
}
