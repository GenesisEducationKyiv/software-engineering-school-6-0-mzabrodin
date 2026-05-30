package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github-release-notifier/internal/domain"
	"github-release-notifier/internal/metrics"
)

const (
	apiBaseURL        = "https://api.github.com"
	apiVersion        = "2026-03-10"
	defaultTimeout    = 10 * time.Second
	keyPrefixExists   = "github:repo_exists:"
	keyPrefixRelease  = "github:latest_release:"
	cacheNoRelease    = "none"
	cacheTrue         = "1"
	cacheFalse        = "0"
	defaultRetryAfter = time.Minute
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

type cacher interface {
	Get(ctx context.Context, key string) (value string, found bool, err error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

type Client struct {
	http    *http.Client
	token   string
	baseURL string
	cache   cacher
	ttl     time.Duration
	log     *slog.Logger
}

func NewClient(token string, log *slog.Logger) *Client {
	return &Client{
		http:    &http.Client{Timeout: defaultTimeout},
		token:   token,
		baseURL: apiBaseURL,
		log:     log.With("component", "github_client"),
	}
}

func (c *Client) WithCache(ca cacher, ttl time.Duration) *Client {
	c.cache = ca
	c.ttl = ttl

	return c
}

func (c *Client) RepoExists(ctx context.Context, owner, repo string) (exists bool, err error) {
	start := time.Now()
	defer func() {
		metrics.GitHubAPIRequestsTotal.WithLabelValues("repo_exists", metrics.ResultLabel(err)).Inc()
		metrics.GitHubAPIRequestDuration.WithLabelValues("repo_exists").Observe(time.Since(start).Seconds())
	}()

	key := keyPrefixExists + owner + "/" + repo

	if c.cache != nil {
		if val, found, err := c.cache.Get(ctx, key); err != nil {
			c.log.WarnContext(ctx, "cache get failed, falling through to GitHub API", "key", key, "error", err)
		} else if found {
			return val == cacheTrue, nil
		}
	}

	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)

	status, _, err := c.do(ctx, url)
	if err != nil {
		return false, err
	}

	if status == http.StatusNotFound {
		c.cacheString(ctx, key, cacheFalse)

		return false, nil
	}

	if status != http.StatusOK {
		return false, fmt.Errorf("unexpected status %d", status)
	}

	c.cacheString(ctx, key, cacheTrue)

	return true, nil
}

func (c *Client) GetLatestRelease(ctx context.Context, owner, repo string) (release *domain.Release, err error) {
	start := time.Now()
	defer func() {
		metrics.GitHubAPIRequestsTotal.WithLabelValues("latest_release", metrics.ResultLabel(err)).Inc()
		metrics.GitHubAPIRequestDuration.WithLabelValues("latest_release").Observe(time.Since(start).Seconds())
	}()

	key := keyPrefixRelease + owner + "/" + repo

	if c.cache != nil {
		if release, found, err := c.getCachedRelease(ctx, key); found {
			return release, err
		}
	}

	status, body, err := c.do(ctx, fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.baseURL, owner, repo))
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		c.cacheString(ctx, key, cacheNoRelease)
		return nil, domain.ErrNoRelease
	}

	if status != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", status)
	}

	return c.parseAndCacheRelease(ctx, key, body)
}

func (c *Client) getCachedRelease(ctx context.Context, key string) (*domain.Release, bool, error) {
	val, found, err := c.cache.Get(ctx, key)
	if err != nil {
		c.log.WarnContext(ctx, "cache get failed, falling through to GitHub API", "key", key, "error", err)
		return nil, false, nil
	}

	if !found {
		return nil, false, nil
	}

	if val == cacheNoRelease {
		return nil, true, domain.ErrNoRelease
	}

	var r domain.Release
	if err := json.Unmarshal([]byte(val), &r); err != nil {
		c.log.WarnContext(ctx, "failed to unmarshal cached release", "key", key, "error", err)
		return nil, false, nil
	}

	return &r, true, nil
}

func (c *Client) parseAndCacheRelease(ctx context.Context, key string, body []byte) (*domain.Release, error) {
	var r githubRelease
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}

	release := &domain.Release{TagName: r.TagName, HTMLURL: r.HTMLURL}

	data, err := json.Marshal(release)
	if err != nil {
		c.log.WarnContext(ctx, "failed to marshal release for cache", "error", err)
	} else {
		c.cacheString(ctx, key, string(data))
	}

	return release, nil
}

func (c *Client) cacheString(ctx context.Context, key, value string) {
	if c.cache == nil {
		return
	}

	saveCtx := context.WithoutCancel(ctx)

	if err := c.cache.Set(saveCtx, key, value, c.ttl); err != nil {
		c.log.WarnContext(saveCtx, "cache set failed", "key", key, "error", err)
	}
}

func (c *Client) do(ctx context.Context, url string) (statusCode int, responseData []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
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
			c.log.ErrorContext(ctx, "failed to close response body", "error", err)
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
