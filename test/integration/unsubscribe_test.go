//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnsubscribe_Success(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	_, unsubToken := subscribeAndGetTokens(t, srv)

	resp := doRequest(t, http.MethodGet, srv.URL+"/api/unsubscribe/"+unsubToken, "", "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var count int
	row := testPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM subscriptions WHERE unsubscribe_token=$1", unsubToken)
	require.NoError(t, row.Scan(&count))
	assert.Zero(t, count, "subscription should be deleted after /api/unsubscribe")
}

func TestUnsubscribe_InvalidTokenLength_Returns400(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodGet, srv.URL+"/api/unsubscribe/tooshort", "", "")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUnsubscribe_UnknownToken_Returns404(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodGet, srv.URL+"/api/unsubscribe/"+randomHex64(), "", "")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUnsubscribe_AlreadyUnsubscribed_Returns404(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	_, unsubToken := subscribeAndGetTokens(t, srv)

	resp := doRequest(t, http.MethodGet, srv.URL+"/api/unsubscribe/"+unsubToken, "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = doRequest(t, http.MethodGet, srv.URL+"/api/unsubscribe/"+unsubToken, "", "")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
