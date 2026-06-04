package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/domain"
)

func newTestClient(serverURL, token string) *Client {
	return &Client{
		http:    &http.Client{Timeout: 5 * time.Second},
		token:   token,
		baseURL: serverURL,
	}
}

// statusServer creates a test server that always responds with status, registers cleanup, and returns a client.
func statusServer(t *testing.T, status int) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)

	return newTestClient(srv.URL, "")
}

// responseServer creates a test server that responds with status and a JSON body, registers cleanup, and returns a client.
func responseServer(t *testing.T, status int, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	return newTestClient(srv.URL, "")
}

// failGetClient creates a client backed by a cache that always fails on Get, using a server that delegates to handler and counts calls via the returned pointer.
func failGetClient(t *testing.T, handler http.HandlerFunc) (client *Client, calls *int) {
	t.Helper()
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	mc := &mockCache{}
	mc.On("Get", mock.Anything, mock.Anything).Return("", false, errors.New("cache get error"))
	mc.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	return newTestClient(srv.URL, "").WithCache(mc, time.Minute), &n
}

type mockCache struct {
	mock.Mock
}

func (m *mockCache) Get(ctx context.Context, key string) (value string, found bool, err error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Bool(1), args.Error(2)
}

func (m *mockCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return m.Called(ctx, key, value, ttl).Error(0)
}

type ClientSuite struct {
	suite.Suite
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

// region RepoExists

func (s *ClientSuite) TestRepoExists_StatusCases() {
	cases := []struct {
		name      string
		status    int
		wantExist bool
		wantErr   bool
		wantErrIs error
	}{
		{"found", http.StatusOK, true, false, nil},
		{"not found", http.StatusNotFound, false, false, nil},
		{"unauthorized", http.StatusUnauthorized, false, true, domain.ErrUnauthorized},
		{"unexpected", http.StatusInternalServerError, false, true, nil},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			c := statusServer(s.T(), tc.status)
			exists, err := c.RepoExists(s.T().Context(), "owner", "repo")
			switch {
			case tc.wantErrIs != nil:
				s.ErrorIs(err, tc.wantErrIs)
			case tc.wantErr:
				s.Error(err)
			default:
				s.Require().NoError(err)
				s.Equal(tc.wantExist, exists)
			}
		})
	}
}

func (s *ClientSuite) TestRepoExists_RateLimited_429() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	s.T().Cleanup(srv.Close)

	c := newTestClient(srv.URL, "")
	_, err := c.RepoExists(s.T().Context(), "owner", "repo")
	s.ErrorIs(err, domain.ErrRateLimited)
}

func (s *ClientSuite) TestRateLimited_403WithHeader() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	s.T().Cleanup(srv.Close)
	c := newTestClient(srv.URL, "")

	s.Run("RepoExists", func() {
		_, err := c.RepoExists(s.T().Context(), "owner", "repo")
		s.ErrorIs(err, domain.ErrRateLimited)
	})
	s.Run("GetLatestRelease", func() {
		_, err := c.GetLatestRelease(s.T().Context(), "owner", "repo")
		s.ErrorIs(err, domain.ErrRateLimited)
	})
}

func (s *ClientSuite) TestRepoExists_SetsAuthHeader() {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	s.T().Cleanup(srv.Close)

	c := newTestClient(srv.URL, "mytoken")
	_, _ = c.RepoExists(s.T().Context(), "owner", "repo")
	s.Equal("Bearer mytoken", gotAuth)
}

func (s *ClientSuite) TestRepoExists_CacheHit_True() {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		s.T().Error("HTTP server was called despite cache hit")
	}))
	s.T().Cleanup(srv.Close)

	mc := &mockCache{}
	defer mc.AssertExpectations(s.T())
	mc.On("Get", mock.Anything, keyPrefixExists+"owner/repo").Return("1", true, nil)

	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	exists, err := c.RepoExists(s.T().Context(), "owner", "repo")
	s.Require().NoError(err)
	s.True(exists)
}

func (s *ClientSuite) TestRepoExists_CacheHit_False() {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		s.T().Error("HTTP server was called despite cache hit")
	}))
	s.T().Cleanup(srv.Close)

	mc := &mockCache{}
	defer mc.AssertExpectations(s.T())
	mc.On("Get", mock.Anything, keyPrefixExists+"owner/repo").Return("0", true, nil)

	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	exists, err := c.RepoExists(s.T().Context(), "owner", "repo")
	s.Require().NoError(err)
	s.False(exists)
}

func (s *ClientSuite) TestRepoExists_CachePopulation() {
	cases := []struct {
		name       string
		status     int
		wantCached string
	}{
		{"found caches 1", http.StatusOK, "1"},
		{"not found caches 0", http.StatusNotFound, "0"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			mc := &mockCache{}
			defer mc.AssertExpectations(s.T())
			var capturedVal string
			mc.On("Get", mock.Anything, keyPrefixExists+"owner/repo").Return("", false, nil)
			mc.On("Set", mock.Anything, keyPrefixExists+"owner/repo", mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) { capturedVal = args.String(2) }).
				Return(nil)
			c := statusServer(s.T(), tc.status).WithCache(mc, time.Minute)
			_, _ = c.RepoExists(s.T().Context(), "owner", "repo")
			s.Equal(tc.wantCached, capturedVal)
		})
	}
}

func (s *ClientSuite) TestRepoExists_CacheGetError_FallsThrough() {
	c, calls := failGetClient(s.T(), func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	exists, err := c.RepoExists(s.T().Context(), "owner", "repo")
	s.Require().NoError(err)
	s.True(exists)
	s.Equal(1, *calls)
}

func (s *ClientSuite) TestRepoExists_CacheSetError_StillReturnsResult() {
	mc := &mockCache{}
	defer mc.AssertExpectations(s.T())
	mc.On("Get", mock.Anything, mock.Anything).Return("", false, nil)
	mc.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("cache set error"))

	c := statusServer(s.T(), http.StatusOK).WithCache(mc, time.Minute)
	exists, err := c.RepoExists(s.T().Context(), "owner", "repo")
	s.Require().NoError(err)
	s.True(exists)
}

// endregion RepoExists

// region GetLatestRelease

func (s *ClientSuite) TestGetLatestRelease_StatusCases() {
	const successBody = `{"tag_name":"v1.2.3","html_url":"https://github.com/owner/repo/releases/tag/v1.2.3"}`
	cases := []struct {
		name      string
		status    int
		body      string
		wantTag   string
		wantErrIs error
	}{
		{"success", http.StatusOK, successBody, "v1.2.3", nil},
		{"no release 404", http.StatusNotFound, "", "", domain.ErrNoRelease},
		{"rate limited 429", http.StatusTooManyRequests, "", "", domain.ErrRateLimited},
		{"unauthorized", http.StatusUnauthorized, "", "", domain.ErrUnauthorized},
		{"unexpected", http.StatusServiceUnavailable, "", "", nil},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			c := responseServer(s.T(), tc.status, tc.body)
			rel, err := c.GetLatestRelease(s.T().Context(), "owner", "repo")
			switch {
			case tc.wantTag != "":
				s.Require().NoError(err)
				s.Equal(tc.wantTag, rel.TagName)
			case tc.wantErrIs != nil:
				s.ErrorIs(err, tc.wantErrIs)
			default:
				s.Error(err)
			}
		})
	}
}

func (s *ClientSuite) TestGetLatestRelease_InvalidJSON() {
	c := responseServer(s.T(), http.StatusOK, "not json")
	_, err := c.GetLatestRelease(s.T().Context(), "owner", "repo")
	s.Error(err)
}

func (s *ClientSuite) TestGetLatestRelease_CacheHit() {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		s.T().Error("HTTP server was called despite cache hit")
	}))
	s.T().Cleanup(srv.Close)

	mc := &mockCache{}
	defer mc.AssertExpectations(s.T())
	mc.On("Get", mock.Anything, keyPrefixRelease+"owner/repo").
		Return(`{"TagName":"v2.0.0","HTMLURL":"https://example.com"}`, true, nil)

	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	rel, err := c.GetLatestRelease(s.T().Context(), "owner", "repo")

	s.Require().NoError(err)
	s.Equal("v2.0.0", rel.TagName)
}

func (s *ClientSuite) TestGetLatestRelease_CacheMiss_PopulatesCache() {
	const body = `{"tag_name":"v3.0.0","html_url":"https://example.com"}`
	mc := &mockCache{}
	defer mc.AssertExpectations(s.T())
	mc.On("Get", mock.Anything, keyPrefixRelease+"owner/repo").Return("", false, nil)
	mc.On("Set", mock.Anything, keyPrefixRelease+"owner/repo", mock.Anything, mock.Anything).Return(nil)

	c := responseServer(s.T(), http.StatusOK, body).WithCache(mc, time.Minute)
	_, _ = c.GetLatestRelease(s.T().Context(), "owner", "repo")
}

func (s *ClientSuite) TestGetLatestRelease_NoRelease_CachesSentinel() {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	}))
	s.T().Cleanup(srv.Close)

	mc := &mockCache{}
	defer mc.AssertExpectations(s.T())
	mc.On("Get", mock.Anything, keyPrefixRelease+"owner/repo").Return("", false, nil).Once()
	mc.On("Set", mock.Anything, keyPrefixRelease+"owner/repo", "none", mock.Anything).Return(nil).Once()
	mc.On("Get", mock.Anything, keyPrefixRelease+"owner/repo").Return("none", true, nil).Once()

	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)

	_, err := c.GetLatestRelease(s.T().Context(), "owner", "repo")
	s.Require().ErrorIs(err, domain.ErrNoRelease)

	_, err = c.GetLatestRelease(s.T().Context(), "owner", "repo")
	s.Require().ErrorIs(err, domain.ErrNoRelease)

	s.Equal(1, calls)
}

func (s *ClientSuite) TestGetLatestRelease_CacheCorruptedJSON_FallsThrough() {
	const goodBody = `{"tag_name":"v1.0.0","html_url":"https://example.com"}`
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(goodBody))
	}))
	s.T().Cleanup(srv.Close)

	mc := &mockCache{}
	defer mc.AssertExpectations(s.T())
	mc.On("Get", mock.Anything, keyPrefixRelease+"owner/repo").Return("not valid json", true, nil)

	var repopulated string
	mc.On("Set", mock.Anything, keyPrefixRelease+"owner/repo", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { repopulated = args.String(2) }).
		Return(nil)

	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	rel, err := c.GetLatestRelease(s.T().Context(), "owner", "repo")

	s.Require().NoError(err)
	s.Equal("v1.0.0", rel.TagName)
	s.Equal(1, calls)
	s.NotEqual("not valid json", repopulated, "cache still has corrupt data after fallthrough")
	s.Contains(repopulated, "v1.0.0")
}

func (s *ClientSuite) TestGetLatestRelease_CacheGetError_FallsThrough() {
	const body = `{"tag_name":"v1.0.0","html_url":"https://example.com"}`
	c, calls := failGetClient(s.T(), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	rel, err := c.GetLatestRelease(s.T().Context(), "owner", "repo")

	s.Require().NoError(err)
	s.Equal("v1.0.0", rel.TagName)
	s.Equal(1, *calls)
}

// endregion GetLatestRelease

func (s *ClientSuite) TestParseRetryAfter_Empty() {
	resp := &http.Response{Header: http.Header{}}
	s.Equal(defaultRetryAfter, parseRetryAfter(resp))
}

func (s *ClientSuite) TestParseRetryAfter_ValidSeconds() {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "90")
	s.Equal(90*time.Second, parseRetryAfter(resp))
}

func (s *ClientSuite) TestParseRetryAfter_InvalidString() {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "not-a-number")
	s.Equal(defaultRetryAfter, parseRetryAfter(resp))
}

func (s *ClientSuite) TestParseRetryAfter_RateLimitReset() {
	resp := &http.Response{Header: http.Header{}}
	resetAt := time.Now().Add(2 * time.Minute).Unix()
	resp.Header.Set("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))
	d := parseRetryAfter(resp)
	s.Greater(d, time.Minute)
	s.Less(d, 3*time.Minute)
}
