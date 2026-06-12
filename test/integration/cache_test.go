//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/infrastructure/cache"
)

const cacheTTL = time.Minute

type CacheSuite struct {
	suite.Suite
	ca cache.Cache
}

func TestCacheSuite(t *testing.T) {
	suite.Run(t, new(CacheSuite))
}

func (s *CacheSuite) SetupSuite() {
	var err error
	s.ca, err = cache.NewRedisCache(s.T().Context(), testRedisURL)
	s.Require().NoError(err)
}

func (s *CacheSuite) TearDownSuite() {
	s.Require().NoError(s.ca.Close())
}

func (s *CacheSuite) TestSetGetRoundTrip() {
	ctx := s.T().Context()
	const key, value = "cache:round-trip", "value"

	s.Require().NoError(s.ca.Set(ctx, key, value, cacheTTL))

	val, found, err := s.ca.Get(ctx, key)

	s.Require().NoError(err)
	s.True(found)
	s.Equal(value, val)
}

func (s *CacheSuite) TestSetOverwritesExistingValue() {
	ctx := s.T().Context()
	const key, newValue, oldValue = "cache:overwrite", "new", "old"

	s.Require().NoError(s.ca.Set(ctx, key, oldValue, cacheTTL))
	s.Require().NoError(s.ca.Set(ctx, key, newValue, cacheTTL))

	val, found, err := s.ca.Get(ctx, key)

	s.Require().NoError(err)
	s.True(found)
	s.Equal(newValue, val)
}

func (s *CacheSuite) TestGetMissReturnsNotFound() {
	const key = "cache:does-not-exist"

	val, found, err := s.ca.Get(s.T().Context(), key)

	s.Require().NoError(err)
	s.False(found)
	s.Empty(val)
}

// TestGetAfterClose exercises the graceful-fallthrough error branch the GitHub
// client relies on: an unusable client must return an error, not panic.
func (s *CacheSuite) TestGetAfterClose() {
	const key = "cache:after-close"

	ca, err := cache.NewRedisCache(s.T().Context(), testRedisURL)
	s.Require().NoError(err)
	s.Require().NoError(ca.Close())

	_, _, err = ca.Get(s.T().Context(), key)

	s.Require().Error(err)
}
