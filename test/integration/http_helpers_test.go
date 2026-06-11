//go:build integration

package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	api "github-release-notifier/internal/subscription/adapter/http"
)

// newTestHTTPServer starts a real HTTP server backed by real repositories against testPool.
func newTestHTTPServer(t *testing.T, repoExists bool) *httptest.Server {
	t.Helper()

	uc := newTestUseCases(repoExists)
	handler := api.NewHandler(uc.subscribe, uc.confirm, uc.unsubscribe, uc.list, testLogger)

	srv := httptest.NewServer(api.NewRouter(handler, testAPIKey, testLogger))
	t.Cleanup(srv.Close)
	return srv
}

// doRequest is a thin helper that sends an HTTP request and returns the response.
func doRequest(t *testing.T, method, url, body, apiKey string) *http.Response {
	t.Helper()
	var bodyReader io.Reader = http.NoBody

	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(t.Context(), method, url, bodyReader)
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

// subscribeAndGetTokens subscribes testEmail to testRepoName via HTTP and returns the
// confirmation and unsubscribe tokens by reading them directly from the test database.
func subscribeAndGetTokens(t *testing.T, srv *httptest.Server) (confirmToken, unsubToken string) {
	t.Helper()
	body := `{"email":"` + testEmail + `","repo":"` + testRepoName + `"}`
	resp := doRequest(t, http.MethodPost, srv.URL+"/api/subscribe", body, testAPIKey)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	row := testPool.QueryRow(t.Context(),
		"SELECT confirm_token, unsubscribe_token FROM subscriptions WHERE email=$1", testEmail)
	require.NoError(t, row.Scan(&confirmToken, &unsubToken))

	return
}

// decodeJSON decodes a JSON response into the destination object and fails the test if an error occurs.
func decodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}
