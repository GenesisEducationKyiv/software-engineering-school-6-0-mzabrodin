//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealth(t *testing.T) {
	srv := newTestServer(t, true)

	resp := doRequest(t, http.MethodGet, srv.URL+"/health", "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]string
	decodeJSON(t, resp, &body)
	assert.Equal(t, "ok", body["status"])
}
