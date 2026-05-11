package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github-release-notifier/internal/domain"
)

func newTestClient(serverURL, token string) *Client {
	return &Client{
		http:    &http.Client{Timeout: 5 * time.Second},
		token:   token,
		baseURL: serverURL,
	}
}

type mockCache struct {
	data map[string]string
}

func newMockCache() *mockCache {
	return &mockCache{data: make(map[string]string)}
}

func (m *mockCache) Get(_ context.Context, key string) (value string, found bool, err error) {
	val, ok := m.data[key]
	return val, ok, nil
}

func (m *mockCache) Set(_ context.Context, key, value string, _ time.Duration) error {
	m.data[key] = value
	return nil
}

func TestRepoExists_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "")
	exists, err := c.RepoExists(context.Background(), "owner", "repo")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRepoExists_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "")
	exists, err := c.RepoExists(context.Background(), "owner", "repo")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRepoExists_RateLimited_429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "")
	_, err := c.RepoExists(context.Background(), "owner", "repo")
	assert.ErrorIs(t, err, domain.ErrRateLimited)
}

func TestRepoExists_RateLimited_403WithHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "")
	_, err := c.RepoExists(context.Background(), "owner", "repo")
	assert.ErrorIs(t, err, domain.ErrRateLimited)
}

func TestRepoExists_UnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "")
	_, err := c.RepoExists(context.Background(), "owner", "repo")
	assert.Error(t, err)
}

func TestRepoExists_SetsAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "mytoken")
	_, _ = c.RepoExists(context.Background(), "owner", "repo")
	assert.Equal(t, "Bearer mytoken", gotAuth)
}

func TestGetLatestRelease_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3","html_url":"https://github.com/owner/repo/releases/tag/v1.2.3"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "")
	rel, err := c.GetLatestRelease(context.Background(), "owner", "repo")
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", rel.TagName)
	assert.Equal(t, "https://github.com/owner/repo/releases/tag/v1.2.3", rel.HTMLURL)
}

func TestGetLatestRelease_NoRelease_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "")
	_, err := c.GetLatestRelease(context.Background(), "owner", "repo")
	assert.ErrorIs(t, err, domain.ErrNoRelease)
}

func TestGetLatestRelease_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "")
	_, err := c.GetLatestRelease(context.Background(), "owner", "repo")
	assert.ErrorIs(t, err, domain.ErrRateLimited)
}

func TestGetLatestRelease_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "")
	_, err := c.GetLatestRelease(context.Background(), "owner", "repo")
	assert.Error(t, err)
}

func TestGetLatestRelease_UnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "")
	_, err := c.GetLatestRelease(context.Background(), "owner", "repo")
	assert.Error(t, err)
}

func TestRepoExists_CacheHit_True(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("HTTP server was called despite cache hit")
	}))
	defer srv.Close()

	mc := newMockCache()
	mc.data[keyPrefixExists+"owner/repo"] = "1"

	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	exists, err := c.RepoExists(context.Background(), "owner", "repo")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRepoExists_CacheHit_False(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("HTTP server was called despite cache hit")
	}))
	defer srv.Close()

	mc := newMockCache()
	mc.data[keyPrefixExists+"owner/repo"] = "0"

	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	exists, err := c.RepoExists(context.Background(), "owner", "repo")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRepoExists_CacheMiss_PopulatesCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mc := newMockCache()
	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	_, _ = c.RepoExists(context.Background(), "owner", "repo")

	assert.Equal(t, "1", mc.data[keyPrefixExists+"owner/repo"])
}

func TestRepoExists_NotFound_PopulatesCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	mc := newMockCache()
	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	_, _ = c.RepoExists(context.Background(), "owner", "repo")

	assert.Equal(t, "0", mc.data[keyPrefixExists+"owner/repo"])
}

func TestGetLatestRelease_CacheHit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("HTTP server was called despite cache hit")
	}))
	defer srv.Close()

	mc := newMockCache()
	mc.data[keyPrefixRelease+"owner/repo"] = `{"TagName":"v2.0.0","HTMLURL":"https://example.com"}`

	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	rel, err := c.GetLatestRelease(context.Background(), "owner", "repo")
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", rel.TagName)
}

func TestGetLatestRelease_CacheMiss_PopulatesCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v3.0.0","html_url":"https://example.com"}`))
	}))
	defer srv.Close()

	mc := newMockCache()
	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	_, _ = c.GetLatestRelease(context.Background(), "owner", "repo")

	assert.Contains(t, mc.data, keyPrefixRelease+"owner/repo")
}

func TestGetLatestRelease_NoRelease_CachesSentinel(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	mc := newMockCache()
	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)

	_, err := c.GetLatestRelease(context.Background(), "owner", "repo")
	require.ErrorIs(t, err, domain.ErrNoRelease)

	_, err = c.GetLatestRelease(context.Background(), "owner", "repo")
	require.ErrorIs(t, err, domain.ErrNoRelease)

	assert.Equal(t, 1, calls)
}

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
