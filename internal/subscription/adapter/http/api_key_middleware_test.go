package http_api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	api "github-release-notifier/internal/subscription/adapter/http"
)

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func newRequest(t *testing.T) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
}

type KeyAuthSuite struct {
	suite.Suite
}

func TestKeyAuthSuite(t *testing.T) {
	suite.Run(t, new(KeyAuthSuite))
}

func (s *KeyAuthSuite) TestKeyAuth() {
	cases := []struct {
		name       string
		key        string
		headerKey  string
		wantStatus int
	}{
		{"no key configured allows all", "", "", http.StatusOK},
		{"correct key allows", "secret", "secret", http.StatusOK},
		{"wrong key returns 401", "secret", "wrong", http.StatusUnauthorized},
		{"missing header returns 401", "secret", "", http.StatusUnauthorized},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			handler := api.NewKeyAuthMiddleware(tc.key)(http.HandlerFunc(okHandler))
			r := newRequest(s.T())
			if tc.headerKey != "" {
				r.Header.Set("X-API-Key", tc.headerKey)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			s.Equal(tc.wantStatus, w.Code)
		})
	}
}
