//go:build integration

package integration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/adapter/cache"
	"github-release-notifier/internal/adapter/github"
	"github-release-notifier/internal/entity"
)

type GitHubClientCacheSuite struct {
	suite.Suite
	ca cache.Cache
}

func TestGitHubClientCacheSuite(t *testing.T) {
	suite.Run(t, new(GitHubClientCacheSuite))
}

func (s *GitHubClientCacheSuite) SetupSuite() {
	var err error
	s.ca, err = cache.NewRedisCache(s.T().Context(), testRedisURL)
	s.Require().NoError(err)
}

func (s *GitHubClientCacheSuite) TearDownSuite() {
	s.Require().NoError(s.ca.Close())
}

func (s *GitHubClientCacheSuite) client() *github.Client {
	return github.NewClient("", testLogger).WithCache(s.ca, time.Minute)
}

func (s *GitHubClientCacheSuite) TestRepoExists_CacheHit_True() {
	ctx := s.T().Context()
	s.Require().NoError(s.ca.Set(ctx, "github:repo_exists:owner/repo-exists-true", "1", time.Minute))

	exists, err := s.client().RepoExists(ctx, "owner", "repo-exists-true")

	s.Require().NoError(err)
	s.True(exists)
}

func (s *GitHubClientCacheSuite) TestRepoExists_CacheHit_False() {
	ctx := s.T().Context()
	s.Require().NoError(s.ca.Set(ctx, "github:repo_exists:owner/repo-exists-false", "0", time.Minute))

	exists, err := s.client().RepoExists(ctx, "owner", "repo-exists-false")

	s.Require().NoError(err)
	s.False(exists)
}

func (s *GitHubClientCacheSuite) TestGetLatestRelease_CacheHit() {
	ctx := s.T().Context()
	release := entity.Release{TagName: "v3.0.0", HTMLURL: "https://github.com/owner/repo-release/releases/tag/v3.0.0"}
	data, err := json.Marshal(release)
	s.Require().NoError(err)
	s.Require().NoError(s.ca.Set(ctx, "github:latest_release:owner/repo-release", string(data), time.Minute))

	rel, err := s.client().GetLatestRelease(ctx, "owner", "repo-release")

	s.Require().NoError(err)
	s.Equal("v3.0.0", rel.TagName)
	s.Equal(release.HTMLURL, rel.HTMLURL)
}

func (s *GitHubClientCacheSuite) TestGetLatestRelease_NoReleaseSentinel() {
	ctx := s.T().Context()
	s.Require().NoError(s.ca.Set(ctx, "github:latest_release:owner/repo-no-release", "none", time.Minute))

	_, err := s.client().GetLatestRelease(ctx, "owner", "repo-no-release")

	s.ErrorIs(err, entity.ErrNoRelease)
}
