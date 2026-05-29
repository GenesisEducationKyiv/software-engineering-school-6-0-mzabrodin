//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github-release-notifier/internal/api"
	"github-release-notifier/internal/repository"
	"github-release-notifier/internal/service"
	"github-release-notifier/internal/urlbuilder"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

const (
	testAPIKey   = "test-api-key"
	testBaseURL  = "http://test.example.com"
	testEmail    = "user@example.com"
	testRepoName = "owner/repo"
)

type mockGitHub struct{ mock.Mock }

func (m *mockGitHub) RepoExists(ctx context.Context, owner, repo string) (bool, error) {
	args := m.Called(ctx, owner, repo)
	return args.Bool(0), args.Error(1)
}

type mockConfirmationNotifier struct{ mock.Mock }

func (m *mockConfirmationNotifier) SendConfirmation(email, repo, url string) error {
	return m.Called(email, repo, url).Error(0)
}

func (m *mockConfirmationNotifier) Shutdown() {}

// newTestServer builds a real HTTP server backed by real repositories against testPool.
func newTestServer(t *testing.T, repoExists bool) *httptest.Server {
	t.Helper()

	gh := &mockGitHub{}
	gh.On("RepoExists", mock.Anything, mock.Anything, mock.Anything).Return(repoExists, nil).Maybe()

	notifier := &mockConfirmationNotifier{}
	notifier.On("SendConfirmation", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	repos := repository.NewGitHubRepoRepository(testPool)
	subs := repository.NewSubscriptionRepository(testPool)
	urls := urlbuilder.New(testBaseURL)
	svc := service.NewSubscriptionService(repos, subs, gh, notifier, urls, testLogger)
	srv := httptest.NewServer(api.NewRouter(api.NewHandler(svc, testLogger), testAPIKey, testLogger))
	t.Cleanup(srv.Close)
	return srv
}

// truncateAll removes all rows between tests so each test starts with a clean DB.
func truncateAll(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "TRUNCATE subscriptions, repositories CASCADE")
	require.NoError(t, err)
}

// doRequest is a thin helper that sends an HTTP request and returns the response.
func doRequest(t *testing.T, method, url, body, apiKey string) *http.Response {
	t.Helper()
	var bodyReader io.Reader = http.NoBody

	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, url, bodyReader)
	require.NoError(t, err)

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	})

	return resp
}

// subscribeAndGetTokens subscribes testEmail to testRepoName and returns the confirmation
// and unsubscribe tokens by reading them directly from the test database.
func subscribeAndGetTokens(t *testing.T, srv *httptest.Server) (confirmToken, unsubToken string) {
	t.Helper()
	body := `{"email":"` + testEmail + `","repo":"` + testRepoName + `"}`
	resp := doRequest(t, http.MethodPost, srv.URL+"/api/subscribe", body, testAPIKey)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	row := testPool.QueryRow(context.Background(),
		"SELECT confirm_token, unsubscribe_token FROM subscriptions WHERE email=$1", testEmail)
	require.NoError(t, row.Scan(&confirmToken, &unsubToken))

	return
}

// decodeJSON decodes a JSON response into the destination object and fails the test if an error occurs.
func decodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// randomHex64 returns a syntactically valid 64-char hex token that does not exist in the DB.
func randomHex64() string {
	return strings.Repeat("ab", 32)
}
