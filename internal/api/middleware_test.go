package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/api"
)

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func newRequest() *http.Request {
	return httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
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
			handler := api.KeyAuth(tc.key)(http.HandlerFunc(okHandler))
			r := newRequest()
			if tc.headerKey != "" {
				r.Header.Set("X-API-Key", tc.headerKey)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			s.Equal(tc.wantStatus, w.Code)
		})
	}
}
