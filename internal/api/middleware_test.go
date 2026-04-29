package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func newRequest(method, target string) *http.Request {
	return httptest.NewRequestWithContext(context.Background(), method, target, nil)
}

func TestAPIKeyAuth_NoKeyConfigured_AllowsAll(t *testing.T) {
	handler := KeyAuth("")(http.HandlerFunc(okHandler))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newRequest(http.MethodGet, "/"))

	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}

func TestAPIKeyAuth_CorrectKey_Allows(t *testing.T) {
	handler := KeyAuth("secret")(http.HandlerFunc(okHandler))

	r := newRequest(http.MethodGet, "/")
	r.Header.Set("X-API-Key", "secret")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}

func TestAPIKeyAuth_WrongKey_Returns401(t *testing.T) {
	handler := KeyAuth("secret")(http.HandlerFunc(okHandler))

	r := newRequest(http.MethodGet, "/")
	r.Header.Set("X-API-Key", "wrong")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestAPIKeyAuth_MissingHeader_Returns401(t *testing.T) {
	handler := KeyAuth("secret")(http.HandlerFunc(okHandler))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newRequest(http.MethodGet, "/"))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}
