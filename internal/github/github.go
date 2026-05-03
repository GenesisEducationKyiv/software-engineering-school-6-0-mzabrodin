package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github-release-notifier/internal/cache"
	"github-release-notifier/internal/domain"
)

const (
	apiBaseURL       = "https://api.github.com"
	apiVersion       = "2026-03-10"
	defaultTimeout   = 10 * time.Second
	keyPrefixExists  = "github:repo_exists:"
	keyPrefixRelease = "github:latest_release:"
	cacheNoRelease   = "none"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

type Client struct {
	http    *http.Client
	token   string
	baseURL string
	cache   cache.Cache
	ttl     time.Duration
}

func NewClient(token string) *Client {
	return &Client{
		http:    &http.Client{Timeout: defaultTimeout},
		token:   token,
		baseURL: apiBaseURL,
	}
}

func (c *Client) WithCache(ca cache.Cache, ttl time.Duration) *Client {
	c.cache = ca
	c.ttl = ttl

	return c
}

func (c *Client) RepoExists(ctx context.Context, owner, repo string) (bool, error) {
	if c.cache != nil {
		key := keyPrefixExists + owner + "/" + repo
		if val, err := c.cache.Get(ctx, key); err == nil {
			return val == "1", nil
		} else if !errors.Is(err, domain.ErrMiss) {
			slog.Warn("cache get failed, falling through to GitHub API", "key", key, "error", err)
		}
	}

	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)

	status, _, err := c.do(ctx, url)
	if err != nil {
		return false, err
	}

	if status == http.StatusNotFound {
		c.cacheString(ctx, keyPrefixExists+owner+"/"+repo, "0")

		return false, nil
	}

	if status != http.StatusOK {
		return false, fmt.Errorf("unexpected status %d", status)
	}

	c.cacheString(ctx, keyPrefixExists+owner+"/"+repo, "1")

	return true, nil
}

func (c *Client) GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error) {
	if c.cache != nil {
		key := keyPrefixRelease + owner + "/" + repo
		if val, err := c.cache.Get(ctx, key); err == nil {
			if val == cacheNoRelease {
				return nil, domain.ErrNoRelease
			}
			var r domain.Release
			if err := json.Unmarshal([]byte(val), &r); err == nil {
				return &r, nil
			}
		} else if !errors.Is(err, domain.ErrMiss) {
			slog.Warn("cache get failed, falling through to GitHub API", "key", key, "error", err)
		}
	}

	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.baseURL, owner, repo)

	status, body, err := c.do(ctx, url)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		c.cacheString(ctx, keyPrefixRelease+owner+"/"+repo, cacheNoRelease)
		return nil, domain.ErrNoRelease
	}

	if status != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", status)
	}

	var r githubRelease
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}

	release := &domain.Release{TagName: r.TagName, HTMLURL: r.HTMLURL}
	if data, err := json.Marshal(release); err == nil {
		c.cacheString(ctx, keyPrefixRelease+owner+"/"+repo, string(data))
	}

	return release, nil
}

func (c *Client) cacheString(ctx context.Context, key, value string) {
	if c.cache == nil {
		return
	}

	saveCtx := context.WithoutCancel(ctx)

	if err := c.cache.Set(saveCtx, key, value, c.ttl); err != nil {
		slog.Warn("cache set failed", "key", key, "error", err)
	}
}

func (c *Client) do(ctx context.Context, url string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("http request: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return 0, nil, domain.ErrUnauthorized
	}

	if isRateLimited(resp) {
		resource := resp.Header.Get("X-RateLimit-Resource")
		retryAfter := parseRetryAfter(resp)
		return 0, nil, fmt.Errorf("%w: resource=%s, retry after %s", domain.ErrRateLimited, resource, retryAfter)
	}

	return resp.StatusCode, body, nil
}

func isRateLimited(resp *http.Response) bool {
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}

	return resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0"
}

func parseRetryAfter(resp *http.Response) time.Duration {
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}

	if resetAt := resp.Header.Get("X-RateLimit-Reset"); resetAt != "" {
		if unix, err := strconv.ParseInt(resetAt, 10, 64); err == nil {
			if until := time.Until(time.Unix(unix, 0)); until > 0 {
				return until
			}
		}
	}

	return defaultRetryAfter
}
