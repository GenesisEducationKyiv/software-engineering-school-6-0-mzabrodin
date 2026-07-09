//go:build integration

package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type HTTPConfirmSuite struct {
	suite.Suite
	srv *httptest.Server
}

func TestHTTPConfirmSuite(t *testing.T) {
	suite.Run(t, new(HTTPConfirmSuite))
}

func (s *HTTPConfirmSuite) SetupTest() {
	truncateAll(s.T())
	s.srv = newTestHTTPServer(s.T(), true)
}

func (s *HTTPConfirmSuite) TestSuccess() {
	confirmToken, _ := subscribeAndGetTokens(s.T(), s.srv)

	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/confirm/"+confirmToken, "", "")
	s.Equal(http.StatusOK, resp.StatusCode)

	var confirmed bool
	row := testPool.QueryRow(s.T().Context(),
		"SELECT confirmed FROM subscriptions WHERE email=$1", testEmail)
	s.Require().NoError(row.Scan(&confirmed))
	s.True(confirmed, "subscription should be marked confirmed after /api/confirm")
}

func (s *HTTPConfirmSuite) TestMalformedToken_Returns400() {
	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/confirm/not-a-jwt", "", "")
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *HTTPConfirmSuite) TestUnknownSubscriber_IsIdempotent() {
	token := confirmTokenFor(s.T(), "stranger@example.com")

	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/confirm/"+token, "", "")
	s.Equal(http.StatusOK, resp.StatusCode)
}

func (s *HTTPConfirmSuite) TestAlreadyConfirmed_IsIdempotent() {
	confirmToken, _ := subscribeAndGetTokens(s.T(), s.srv)

	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/confirm/"+confirmToken, "", "")
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	resp = doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/confirm/"+confirmToken, "", "")
	s.Equal(http.StatusOK, resp.StatusCode)
}
