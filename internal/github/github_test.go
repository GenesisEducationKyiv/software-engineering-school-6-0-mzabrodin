package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestParseRetryAfter_Empty(t *testing.T) {
	d := parseRetryAfter("")
	if d != time.Minute {
		t.Errorf("got %v, want 1m", d)
	}
}

func TestParseRetryAfter_ValidSeconds(t *testing.T) {
	d := parseRetryAfter("90")
	if d != 90*time.Second {
		t.Errorf("got %v, want 90s", d)
	}
}

func TestParseRetryAfter_InvalidString(t *testing.T) {
	d := parseRetryAfter("not-a-number")
	if d != time.Minute {
		t.Errorf("got %v, want 1m fallback", d)
	}
}
