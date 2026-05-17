package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func newRequest() *http.Request {
	return httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
}

func securedHandler() http.Handler {
	return KeyAuth("secret")(http.HandlerFunc(okHandler))
}

func TestAPIKeyAuth_NoKeyConfigured_AllowsAll(t *testing.T) {
	handler := KeyAuth("")(http.HandlerFunc(okHandler))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newRequest())

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPIKeyAuth_CorrectKey_Allows(t *testing.T) {
	r := newRequest()
	r.Header.Set("X-API-Key", "secret")
	w := httptest.NewRecorder()
	securedHandler().ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPIKeyAuth_WrongKey_Returns401(t *testing.T) {
	r := newRequest()
	r.Header.Set("X-API-Key", "wrong")
	w := httptest.NewRecorder()
	securedHandler().ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAPIKeyAuth_MissingHeader_Returns401(t *testing.T) {
	w := httptest.NewRecorder()
	securedHandler().ServeHTTP(w, newRequest())

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
