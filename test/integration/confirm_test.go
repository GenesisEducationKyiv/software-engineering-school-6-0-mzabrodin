//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfirm_Success(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	confirmToken, _ := subscribeAndGetTokens(t, srv)

	resp := doRequest(t, http.MethodGet, srv.URL+"/api/confirm/"+confirmToken, "", "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var confirmed bool
	row := testPool.QueryRow(context.Background(),
		"SELECT confirmed FROM subscriptions WHERE confirm_token=$1", confirmToken)
	require.NoError(t, row.Scan(&confirmed))
	assert.True(t, confirmed, "subscription should be marked confirmed after /api/confirm")
}

func TestConfirm_InvalidTokenLength_Returns400(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodGet, srv.URL+"/api/confirm/tooshort", "", "")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestConfirm_UnknownToken_Returns404(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodGet, srv.URL+"/api/confirm/"+randomHex64(), "", "")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestConfirm_AlreadyConfirmed_IsIdempotent(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	confirmToken, _ := subscribeAndGetTokens(t, srv)

	resp := doRequest(t, http.MethodGet, srv.URL+"/api/confirm/"+confirmToken, "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = doRequest(t, http.MethodGet, srv.URL+"/api/confirm/"+confirmToken, "", "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
