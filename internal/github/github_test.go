package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github-release-notifier/internal/domain"
)

var ctx = context.Background()

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
func failGetClient(t *testing.T, handler http.HandlerFunc) (*Client, *int) {
	t.Helper()
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	mc := &mockCache{data: make(map[string]string), failOnGet: true}

	return newTestClient(srv.URL, "").WithCache(mc, time.Minute), &n
}

type mockCache struct {
	data       map[string]string
	failOnGet  bool
	failOnSet  bool
	lastSetCtx context.Context
}

func newMockCache() *mockCache {
	return &mockCache{data: make(map[string]string)}
}

func (m *mockCache) Get(_ context.Context, key string) (value string, found bool, err error) {
	if m.failOnGet {
		return "", false, errors.New("cache get error")
	}
	val, ok := m.data[key]

	return val, ok, nil
}

func (m *mockCache) Set(ctx context.Context, key, value string, _ time.Duration) error {
	m.lastSetCtx = ctx
	if m.failOnSet {
		return errors.New("cache set error")
	}
	m.data[key] = value

	return nil
}

// region RepoExists

func TestRepoExists_StatusCases(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			c := statusServer(t, tc.status)
			exists, err := c.RepoExists(ctx, "owner", "repo")
			switch {
			case tc.wantErrIs != nil:
				assert.ErrorIs(t, err, tc.wantErrIs)

			case tc.wantErr:
				assert.Error(t, err)

			default:
				require.NoError(t, err)
				assert.Equal(t, tc.wantExist, exists)
			}
		})
	}
}

func TestRepoExists_RateLimited_429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(srv.URL, "")
	_, err := c.RepoExists(ctx, "owner", "repo")
	assert.ErrorIs(t, err, domain.ErrRateLimited)
}

func TestRepoExists_RateLimited_403WithHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(srv.URL, "")
	_, err := c.RepoExists(ctx, "owner", "repo")
	assert.ErrorIs(t, err, domain.ErrRateLimited)
}

func TestRepoExists_SetsAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(srv.URL, "mytoken")
	_, _ = c.RepoExists(ctx, "owner", "repo")
	assert.Equal(t, "Bearer mytoken", gotAuth)
}

func TestRepoExists_CacheHit_True(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("HTTP server was called despite cache hit")
	}))
	t.Cleanup(srv.Close)

	mc := newMockCache()
	mc.data[keyPrefixExists+"owner/repo"] = "1"

	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	exists, err := c.RepoExists(ctx, "owner", "repo")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRepoExists_CacheHit_False(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("HTTP server was called despite cache hit")
	}))
	t.Cleanup(srv.Close)

	mc := newMockCache()
	mc.data[keyPrefixExists+"owner/repo"] = "0"

	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	exists, err := c.RepoExists(ctx, "owner", "repo")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRepoExists_CacheMiss_PopulatesCache(t *testing.T) {
	mc := newMockCache()
	c := statusServer(t, http.StatusOK)
	c = c.WithCache(mc, time.Minute)
	_, _ = c.RepoExists(ctx, "owner", "repo")
	assert.Equal(t, "1", mc.data[keyPrefixExists+"owner/repo"])
}

func TestRepoExists_NotFound_PopulatesCache(t *testing.T) {
	mc := newMockCache()
	c := statusServer(t, http.StatusNotFound)
	c = c.WithCache(mc, time.Minute)
	_, _ = c.RepoExists(ctx, "owner", "repo")
	assert.Equal(t, "0", mc.data[keyPrefixExists+"owner/repo"])
}

func TestRepoExists_CacheGetError_FallsThrough(t *testing.T) {
	c, calls := failGetClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	exists, err := c.RepoExists(ctx, "owner", "repo")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, 1, *calls)
}

func TestRepoExists_CacheSetError_StillReturnsResult(t *testing.T) {
	mc := &mockCache{data: make(map[string]string), failOnSet: true}
	c := statusServer(t, http.StatusOK).WithCache(mc, time.Minute)
	exists, err := c.RepoExists(ctx, "owner", "repo")
	require.NoError(t, err)
	assert.True(t, exists)
}

// endregion RepoExists

// region GetLatestRelease

func TestGetLatestRelease_StatusCases(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			c := responseServer(t, tc.status, tc.body)
			rel, err := c.GetLatestRelease(ctx, "owner", "repo")
			switch {
			case tc.wantTag != "":
				require.NoError(t, err)
				assert.Equal(t, tc.wantTag, rel.TagName)

			case tc.wantErrIs != nil:
				assert.ErrorIs(t, err, tc.wantErrIs)

			default:
				assert.Error(t, err)
			}
		})
	}
}

func TestGetLatestRelease_RateLimited_403WithHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(srv.URL, "")
	_, err := c.GetLatestRelease(ctx, "owner", "repo")

	assert.ErrorIs(t, err, domain.ErrRateLimited)
}

func TestGetLatestRelease_InvalidJSON(t *testing.T) {
	c := responseServer(t, http.StatusOK, "not json")
	_, err := c.GetLatestRelease(ctx, "owner", "repo")
	assert.Error(t, err)
}

func TestGetLatestRelease_CacheHit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("HTTP server was called despite cache hit")
	}))
	t.Cleanup(srv.Close)

	mc := newMockCache()
	mc.data[keyPrefixRelease+"owner/repo"] = `{"TagName":"v2.0.0","HTMLURL":"https://example.com"}`

	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	rel, err := c.GetLatestRelease(ctx, "owner", "repo")

	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", rel.TagName)
}

func TestGetLatestRelease_CacheMiss_PopulatesCache(t *testing.T) {
	const body = `{"tag_name":"v3.0.0","html_url":"https://example.com"}`
	mc := newMockCache()
	c := responseServer(t, http.StatusOK, body)
	c = c.WithCache(mc, time.Minute)
	_, _ = c.GetLatestRelease(ctx, "owner", "repo")
	assert.Contains(t, mc.data, keyPrefixRelease+"owner/repo")
}

func TestGetLatestRelease_NoRelease_CachesSentinel(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	mc := newMockCache()
	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)

	_, err := c.GetLatestRelease(ctx, "owner", "repo")
	require.ErrorIs(t, err, domain.ErrNoRelease)

	_, err = c.GetLatestRelease(ctx, "owner", "repo")
	require.ErrorIs(t, err, domain.ErrNoRelease)

	assert.Equal(t, 1, calls)
}

func TestGetLatestRelease_CacheCorruptedJSON_FallsThrough(t *testing.T) {
	const goodBody = `{"tag_name":"v1.0.0","html_url":"https://example.com"}`
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(goodBody))
	}))
	t.Cleanup(srv.Close)

	mc := newMockCache()
	mc.data[keyPrefixRelease+"owner/repo"] = "not valid json"

	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	rel, err := c.GetLatestRelease(ctx, "owner", "repo")

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", rel.TagName)
	assert.Equal(t, 1, calls)

	// The corrupt entry must be replaced with fresh data so the next call is served from cache.
	repopulated := mc.data[keyPrefixRelease+"owner/repo"]

	assert.NotEqual(t, "not valid json", repopulated, "cache still has corrupt data after fallthrough")
	assert.Contains(t, repopulated, "v1.0.0")
}

func TestGetLatestRelease_CacheGetError_FallsThrough(t *testing.T) {
	const body = `{"tag_name":"v1.0.0","html_url":"https://example.com"}`
	c, calls := failGetClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	rel, err := c.GetLatestRelease(ctx, "owner", "repo")

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", rel.TagName)
	assert.Equal(t, 1, *calls)
}

// endregion GetLatestRelease

func TestParseRetryAfter_Empty(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	assert.Equal(t, defaultRetryAfter, parseRetryAfter(resp))
}

func TestParseRetryAfter_ValidSeconds(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "90")
	assert.Equal(t, 90*time.Second, parseRetryAfter(resp))
}

func TestParseRetryAfter_InvalidString(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "not-a-number")
	assert.Equal(t, defaultRetryAfter, parseRetryAfter(resp))
}

func TestParseRetryAfter_RateLimitReset(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resetAt := time.Now().Add(2 * time.Minute).Unix()
	resp.Header.Set("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))
	d := parseRetryAfter(resp)
	assert.Greater(t, d, time.Minute)
	assert.Less(t, d, 3*time.Minute)
}
