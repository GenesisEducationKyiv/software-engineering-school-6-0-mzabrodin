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
)

func newTestHTTPServer(t *testing.T, repoExists bool) *httptest.Server {
	t.Helper()
	return newTestServer(t, repoExists)
}

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
		req.Header.Set("Authorization", "Bearer "+apiKey)
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

func decodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}
