//go:build integration

package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type HTTPHealthSuite struct {
	suite.Suite
	srv *httptest.Server
}

func TestHTTPHealthSuite(t *testing.T) {
	suite.Run(t, new(HTTPHealthSuite))
}

func (s *HTTPHealthSuite) SetupTest() {
	truncateAll(s.T())
	s.srv = newTestHTTPServer(s.T(), true)
}

func (s *HTTPHealthSuite) TestHealth() {
	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/health", "", "")
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var body map[string]string
	decodeJSON(s.T(), resp, &body)
	s.Equal("ok", body["status"])
}
