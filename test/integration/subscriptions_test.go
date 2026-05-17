//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSubscriptions_NoAPIKey_Returns401(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodGet, srv.URL+"/api/subscriptions?email="+testEmail, "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestGetSubscriptions_MissingEmailParam_Returns400(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodGet, srv.URL+"/api/subscriptions", "", testAPIKey)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetSubscriptions_InvalidEmail_Returns400(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodGet, srv.URL+"/api/subscriptions?email=notanemail", "", testAPIKey)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetSubscriptions_EmptyList(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodGet, srv.URL+"/api/subscriptions?email="+testEmail, "", testAPIKey)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	decodeJSON(t, resp, &result)
	assert.Empty(t, result)
}

func TestSubscribeConfirmList_EndToEnd(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	confirmToken, _ := subscribeAndGetTokens(t, srv)

	resp := doRequest(t, http.MethodGet, srv.URL+"/api/subscriptions?email="+testEmail, "", testAPIKey)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assertSingleSubscription(t, resp, false)

	resp = doRequest(t, http.MethodGet, srv.URL+"/api/confirm/"+confirmToken, "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = doRequest(t, http.MethodGet, srv.URL+"/api/subscriptions?email="+testEmail, "", testAPIKey)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assertSingleSubscription(t, resp, true)
}

func assertSingleSubscription(t *testing.T, resp *http.Response, confirmed bool) {
	t.Helper()
	var result []map[string]any
	decodeJSON(t, resp, &result)
	require.Len(t, result, 1)
	assert.Equal(t, testEmail, result[0]["email"])
	assert.Equal(t, testRepoName, result[0]["repo"])
	assert.Equal(t, confirmed, result[0]["confirmed"])
}
