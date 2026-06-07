//go:build integration

package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type HTTPUnsubscribeSuite struct {
	suite.Suite
	srv *httptest.Server
}

func TestHTTPUnsubscribeSuite(t *testing.T) {
	suite.Run(t, new(HTTPUnsubscribeSuite))
}

func (s *HTTPUnsubscribeSuite) SetupTest() {
	truncateAll(s.T())
	s.srv = newTestHTTPServer(s.T(), true)
}

func (s *HTTPUnsubscribeSuite) TestSuccess() {
	_, unsubToken := subscribeAndGetTokens(s.T(), s.srv)

	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/unsubscribe/"+unsubToken, "", "")
	s.Equal(http.StatusOK, resp.StatusCode)

	var count int
	row := testPool.QueryRow(s.T().Context(),
		"SELECT COUNT(*) FROM subscriptions WHERE unsubscribe_token=$1", unsubToken)
	s.Require().NoError(row.Scan(&count))
	s.Zero(count, "subscription should be deleted after /api/unsubscribe")
}

func (s *HTTPUnsubscribeSuite) TestInvalidTokenLength_Returns400() {
	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/unsubscribe/tooshort", "", "")
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *HTTPUnsubscribeSuite) TestUnknownToken_Returns404() {
	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/unsubscribe/"+randomHex64(), "", "")
	s.Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *HTTPUnsubscribeSuite) TestAlreadyUnsubscribed_Returns404() {
	_, unsubToken := subscribeAndGetTokens(s.T(), s.srv)

	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/unsubscribe/"+unsubToken, "", "")
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	resp = doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/unsubscribe/"+unsubToken, "", "")
	s.Equal(http.StatusNotFound, resp.StatusCode)
}
