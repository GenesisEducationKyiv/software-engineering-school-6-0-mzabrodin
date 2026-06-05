package cache_test

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/cache"
)

const cacheTTL = time.Minute

type CacheUnitSuite struct {
	suite.Suite
	mr *miniredis.Miniredis
	ca cache.Cache
}

func TestCacheUnitSuite(t *testing.T) {
	suite.Run(t, new(CacheUnitSuite))
}

// SetupTest gives each test a fresh in-process Redis (miniredis) and cache.
// miniredis.RunT registers its own t.Cleanup, so the server is torn down automatically.
func (s *CacheUnitSuite) SetupTest() {
	s.mr = miniredis.RunT(s.T())

	var err error
	s.ca, err = cache.NewRedisCache(s.T().Context(), "redis://"+s.mr.Addr())
	s.Require().NoError(err)
}

func (s *CacheUnitSuite) TearDownTest() {
	s.Require().NoError(s.ca.Close())
}

func (s *CacheUnitSuite) TestNewRedisCacheBadURL() {
	ca, err := cache.NewRedisCache(s.T().Context(), "://not-a-url")

	s.Require().Error(err)
	s.Nil(ca)
}

func (s *CacheUnitSuite) TestNewRedisCachePingFailure() {
	// Nothing listens on port 1, so the ping is refused immediately. max_retries=-1
	// disables go-redis's retry/backoff so the failure surfaces without delay.
	ca, err := cache.NewRedisCache(s.T().Context(), "redis://localhost:1?dial_timeout=200ms&max_retries=-1")

	s.Require().Error(err)
	s.Nil(ca)
}

func (s *CacheUnitSuite) TestSetGetRoundTrip() {
	ctx := s.T().Context()
	const key, value = "cache:round-trip", "value"

	s.Require().NoError(s.ca.Set(ctx, key, value, cacheTTL))

	val, found, err := s.ca.Get(ctx, key)

	s.Require().NoError(err)
	s.True(found)
	s.Equal(value, val)
}

func (s *CacheUnitSuite) TestSetOverwritesExistingValue() {
	ctx := s.T().Context()
	const key, newValue, oldValue = "cache:overwrite", "new", "old"

	s.Require().NoError(s.ca.Set(ctx, key, oldValue, cacheTTL))
	s.Require().NoError(s.ca.Set(ctx, key, newValue, cacheTTL))

	val, found, err := s.ca.Get(ctx, key)

	s.Require().NoError(err)
	s.True(found)
	s.Equal(newValue, val)
}

func (s *CacheUnitSuite) TestGetMissReturnsNotFound() {
	const key = "cache:does-not-exist"

	val, found, err := s.ca.Get(s.T().Context(), key)

	s.Require().NoError(err)
	s.False(found)
	s.Empty(val)
}

func (s *CacheUnitSuite) TestGetExpiredKeyReturnsNotFound() {
	ctx := s.T().Context()
	const key, value = "cache:expiring", "value"

	s.Require().NoError(s.ca.Set(ctx, key, value, cacheTTL))

	s.mr.FastForward(cacheTTL + time.Second) // deterministically age out the TTL

	_, found, err := s.ca.Get(ctx, key)

	s.Require().NoError(err)
	s.False(found)
}

// TestGetAfterClose exercises the graceful-fallthrough error branch the GitHub
// client relies on: an unusable client must return an error, not panic. Uses its
// own cache so the suite's shared instance stays open for TearDownTest.
func (s *CacheUnitSuite) TestGetAfterClose() {
	const key = "cache:after-close"

	ca, err := cache.NewRedisCache(s.T().Context(), "redis://"+s.mr.Addr())
	s.Require().NoError(err)
	s.Require().NoError(ca.Close())

	_, _, err = ca.Get(s.T().Context(), key)

	s.Require().Error(err)
}
