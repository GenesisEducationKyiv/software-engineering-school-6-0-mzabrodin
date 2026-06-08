package logging_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/infrastructure/logging"
)

type HTTPRequestIDSuite struct {
	suite.Suite
}

func TestHTTPRequestIDSuite(t *testing.T) {
	suite.Run(t, new(HTTPRequestIDSuite))
}

func (s *HTTPRequestIDSuite) serve(inbound string) (observed, responseHeader string) {
	s.T().Helper()

	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		observed = logging.RequestID(r.Context())
	})

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/", http.NoBody)
	if inbound != "" {
		req.Header.Set("X-Request-Id", inbound)
	}

	rec := httptest.NewRecorder()
	logging.RequestIDMiddleware(next).ServeHTTP(rec, req)

	return observed, rec.Header().Get("X-Request-Id")
}

func (s *HTTPRequestIDSuite) TestGeneratesIDWhenAbsent() {
	observed, responseHeader := s.serve("")

	s.NotEmpty(observed)
	s.Equal(observed, responseHeader)
}

func (s *HTTPRequestIDSuite) TestHonorsInboundHeader() {
	observed, responseHeader := s.serve("fixed-id")

	s.Equal("fixed-id", observed)
	s.Equal("fixed-id", responseHeader)
}
