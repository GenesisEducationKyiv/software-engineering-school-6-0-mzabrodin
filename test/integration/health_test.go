//go:build integration

package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type HealthSuite struct {
	suite.Suite
	srv *httptest.Server
}

func TestHealthSuite(t *testing.T) {
	suite.Run(t, new(HealthSuite))
}

func (s *HealthSuite) SetupTest() {
	truncateAll(s.T())
	s.srv = newTestServer(s.T(), true)
}

func (s *HealthSuite) TestHealth() {
	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/health", "", "")
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var body map[string]string
	decodeJSON(s.T(), resp, &body)
	s.Equal("ok", body["status"])
}
