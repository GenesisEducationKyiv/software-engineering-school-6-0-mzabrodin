package logging_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/infrastructure/logging"
)

type EdgeSuite struct {
	suite.Suite
}

func TestEdgeSuite(t *testing.T) {
	suite.Run(t, new(EdgeSuite))
}

func (s *EdgeSuite) TestDetectProtocol() {
	cases := []struct {
		name        string
		contentType string
		path        string
		want        string
	}{
		{"grpc-web", "application/grpc-web+proto", "/app.v1.SubscriptionService/Subscribe", "grpc-web"},
		{"grpc", "application/grpc", "/app.v1.SubscriptionService/Subscribe", "grpc"},
		{"rest", "application/json", "/api/subscribe", "rest"},
		{"connect", "application/json", "/app.v1.SubscriptionService/Subscribe", "connect"},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, tc.path, http.NoBody)
			req.Header.Set("Content-Type", tc.contentType)

			s.Equal(tc.want, logging.DetectProtocol(req))
		})
	}
}

func (s *EdgeSuite) TestEdgeMiddlewareGeneratesRequestID() {
	var gotID, gotProtocol string

	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotID = logging.RequestID(r.Context())
		gotProtocol = logging.Protocol(r.Context())
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, "/api/subscribe", http.NoBody)
	logging.EdgeMiddleware(next).ServeHTTP(rec, req)

	s.NotEmpty(gotID, "a request id must be generated when absent")
	s.Equal(gotID, rec.Header().Get("X-Request-Id"), "the generated id is echoed back to the client")
	s.Equal("rest", gotProtocol)
}

func (s *EdgeSuite) TestEdgeMiddlewarePreservesIncomingRequestID() {
	var gotID string

	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotID = logging.RequestID(r.Context())
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, "/api/subscribe", http.NoBody)
	req.Header.Set("X-Request-Id", "incoming-id")
	logging.EdgeMiddleware(next).ServeHTTP(rec, req)

	s.Equal("incoming-id", gotID)
	s.Equal("incoming-id", rec.Header().Get("X-Request-Id"))
}
