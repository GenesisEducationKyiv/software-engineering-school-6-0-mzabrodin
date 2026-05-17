//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscribe_Success(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodPost, srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, testAPIKey)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var email, repo string
	var confirmed bool
	row := testPool.QueryRow(context.Background(), `
		SELECT s.email, r.name, s.confirmed
		FROM subscriptions s
		JOIN repositories r ON r.id = s.repository_id
		WHERE s.email = $1`, testEmail)
	require.NoError(t, row.Scan(&email, &repo, &confirmed))
	assert.Equal(t, testEmail, email)
	assert.Equal(t, testRepoName, repo)
	assert.False(t, confirmed, "new subscription must not be confirmed")
}

func TestSubscribe_NoAPIKey_Returns401(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodPost, srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestSubscribe_WrongAPIKey_Returns401(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodPost, srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, "wrong-key")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestSubscribe_InvalidJSON_Returns400(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodPost, srv.URL+"/api/subscribe", "not-json", testAPIKey)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSubscribe_EmptyEmail_Returns400(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodPost, srv.URL+"/api/subscribe",
		`{"email":"","repo":"owner/repo"}`, testAPIKey)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSubscribe_EmptyRepo_Returns400(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodPost, srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":""}`, testAPIKey)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSubscribe_InvalidEmail_Returns400(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodPost, srv.URL+"/api/subscribe",
		`{"email":"notanemail","repo":"owner/repo"}`, testAPIKey)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSubscribe_InvalidRepo_Returns400(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodPost, srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"noslash"}`, testAPIKey)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSubscribe_RepoNotOnGitHub_Returns404(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, false) // mock reports repo doesn't exist

	resp := doRequest(t, http.MethodPost, srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, testAPIKey)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSubscribe_Duplicate_Returns409(t *testing.T) {
	truncateAll(t)
	srv := newTestServer(t, true)

	body := `{"email":"user@example.com","repo":"owner/repo"}`
	doRequest(t, http.MethodPost, srv.URL+"/api/subscribe", body, testAPIKey)

	resp := doRequest(t, http.MethodPost, srv.URL+"/api/subscribe", body, testAPIKey)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}
