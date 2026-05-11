package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !exists {
		t.Error("expected repo to exist")
	}
}

func TestRepoExists_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "")
	exists, err := c.RepoExists(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exists {
		t.Error("expected repo to not exist")
	}
}

func TestRepoExists_RateLimited_429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))

	defer srv.Close()

	c := newTestClient(srv.URL, "")
	_, err := c.RepoExists(context.Background(), "owner", "repo")
	if !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("got %v, want ErrRateLimited", err)
	}
}

func TestRepoExists_RateLimited_403WithHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))

	defer srv.Close()

	c := newTestClient(srv.URL, "")
	_, err := c.RepoExists(context.Background(), "owner", "repo")
	if !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("got %v, want ErrRateLimited", err)
	}
}

func TestRepoExists_UnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	defer srv.Close()

	c := newTestClient(srv.URL, "")
	_, err := c.RepoExists(context.Background(), "owner", "repo")
	if err == nil {
		t.Error("expected error for unexpected status, got nil")
	}
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
	if gotAuth != "Bearer mytoken" {
		t.Errorf("got Authorization %q, want \"Bearer mytoken\"", gotAuth)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rel.TagName != "v1.2.3" {
		t.Errorf("got tag %q, want \"v1.2.3\"", rel.TagName)
	}

	if rel.HTMLURL != "https://github.com/owner/repo/releases/tag/v1.2.3" {
		t.Errorf("unexpected HTMLURL: %q", rel.HTMLURL)
	}
}

func TestGetLatestRelease_NoRelease_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	defer srv.Close()

	c := newTestClient(srv.URL, "")
	_, err := c.GetLatestRelease(context.Background(), "owner", "repo")
	if !errors.Is(err, domain.ErrNoRelease) {
		t.Errorf("got %v, want ErrNoRelease", err)
	}
}

func TestGetLatestRelease_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))

	defer srv.Close()

	c := newTestClient(srv.URL, "")
	_, err := c.GetLatestRelease(context.Background(), "owner", "repo")
	if !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("got %v, want ErrRateLimited", err)
	}
}

func TestGetLatestRelease_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))

	defer srv.Close()

	c := newTestClient(srv.URL, "")
	_, err := c.GetLatestRelease(context.Background(), "owner", "repo")
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestGetLatestRelease_UnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	defer srv.Close()

	c := newTestClient(srv.URL, "")
	_, err := c.GetLatestRelease(context.Background(), "owner", "repo")
	if err == nil {
		t.Error("expected error for unexpected status, got nil")
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected cache hit to return true")
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exists {
		t.Error("expected cache hit to return false")
	}
}

func TestRepoExists_CacheMiss_PopulatesCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	defer srv.Close()

	mc := newMockCache()
	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	_, _ = c.RepoExists(context.Background(), "owner", "repo")

	if got := mc.data[keyPrefixExists+"owner/repo"]; got != "1" {
		t.Errorf("cache entry = %q, want \"1\"", got)
	}
}

func TestRepoExists_NotFound_PopulatesCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	defer srv.Close()

	mc := newMockCache()
	c := newTestClient(srv.URL, "").WithCache(mc, time.Minute)
	_, _ = c.RepoExists(context.Background(), "owner", "repo")

	if got := mc.data[keyPrefixExists+"owner/repo"]; got != "0" {
		t.Errorf("cache entry = %q, want \"0\"", got)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rel.TagName != "v2.0.0" {
		t.Errorf("got tag %q, want \"v2.0.0\"", rel.TagName)
	}
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

	if _, ok := mc.data[keyPrefixRelease+"owner/repo"]; !ok {
		t.Error("expected cache to be populated after API call")
	}
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
	if !errors.Is(err, domain.ErrNoRelease) {
		t.Fatalf("got %v, want ErrNoRelease", err)
	}

	_, err = c.GetLatestRelease(context.Background(), "owner", "repo")
	if !errors.Is(err, domain.ErrNoRelease) {
		t.Fatalf("got %v, want ErrNoRelease", err)
	}

	if calls != 1 {
		t.Errorf("server called %d times, want 1", calls)
	}
}

func TestParseRetryAfter_Empty(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	d := parseRetryAfter(resp)
	if d != defaultRetryAfter {
		t.Errorf("got %v, want %v", d, defaultRetryAfter)
	}
}

func TestParseRetryAfter_ValidSeconds(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "90")
	d := parseRetryAfter(resp)
	if d != 90*time.Second {
		t.Errorf("got %v, want 90s", d)
	}
}

func TestParseRetryAfter_InvalidString(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "not-a-number")
	d := parseRetryAfter(resp)
	if d != defaultRetryAfter {
		t.Errorf("got %v, want %v", d, defaultRetryAfter)
	}
}

func TestParseRetryAfter_RateLimitReset(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resetAt := time.Now().Add(2 * time.Minute).Unix()
	resp.Header.Set("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))
	d := parseRetryAfter(resp)
	if d < time.Minute || d > 3*time.Minute {
		t.Errorf("got %v, want ~2m", d)
	}
}
