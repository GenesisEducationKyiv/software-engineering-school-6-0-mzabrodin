package github

import (
	"context"
	"encoding/json"
	"fmt"
	"github-release-notifier/internal/domain"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const (
	apiBaseURL     = "https://api.github.com"
	apiVersion     = "2026-03-10"
	defaultTimeout = 10 * time.Second
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

type Client struct {
	http    *http.Client
	token   string
	baseURL string
}

func NewClient(token string) *Client {
	return &Client{
		http:    &http.Client{Timeout: defaultTimeout},
		token:   token,
		baseURL: apiBaseURL,
	}
}

func (c *Client) RepoExists(ctx context.Context, owner, repo string) (bool, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)

	status, _, err := c.do(ctx, url)
	if err != nil {
		return false, err
	}

	if status == http.StatusNotFound {
		return false, nil
	}

	if status != http.StatusOK {
		return false, fmt.Errorf("unexpected status %d", status)
	}

	return true, nil
}

func (c *Client) GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.baseURL, owner, repo)

	status, body, err := c.do(ctx, url)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, domain.ErrNoRelease
	}

	if status != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", status)
	}

	var r githubRelease
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}

	return &domain.Release{TagName: r.TagName, HTMLURL: r.HTMLURL}, nil
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

	defer func(body io.ReadCloser) {
		if err := body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return 0, nil, domain.ErrUnauthorized
	}

	if resp.StatusCode == http.StatusTooManyRequests ||
		(resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0") {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))

		return 0, nil, fmt.Errorf("%w, retry after %s", domain.ErrRateLimited, retryAfter)
	}

	return resp.StatusCode, body, nil
}

func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return time.Minute
	}

	seconds, err := strconv.Atoi(header)
	if err != nil {
		return time.Minute
	}

	return time.Duration(seconds) * time.Second
}
