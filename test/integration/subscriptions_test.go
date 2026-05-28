//go:build integration

package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SubscriptionsSuite struct {
	suite.Suite
	srv *httptest.Server
}

func TestSubscriptionsSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionsSuite))
}

func (s *SubscriptionsSuite) SetupTest() {
	truncateAll(s.T())
	s.srv = newTestServer(s.T(), true)
}

func (s *SubscriptionsSuite) TestNoAPIKey_Returns401() {
	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/subscriptions?email="+testEmail, "", "")
	s.Equal(http.StatusUnauthorized, resp.StatusCode)
}

func (s *SubscriptionsSuite) TestMissingEmailParam_Returns400() {
	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/subscriptions", "", testAPIKey)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *SubscriptionsSuite) TestInvalidEmail_Returns400() {
	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/subscriptions?email=notanemail", "", testAPIKey)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *SubscriptionsSuite) TestEmptyList() {
	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/subscriptions?email="+testEmail, "", testAPIKey)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var result []map[string]any
	decodeJSON(s.T(), resp, &result)
	s.Empty(result)
}

func (s *SubscriptionsSuite) TestSubscribeConfirmList_EndToEnd() {
	confirmToken, _ := subscribeAndGetTokens(s.T(), s.srv)

	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/subscriptions?email="+testEmail, "", testAPIKey)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.assertSingleSubscription(resp, false)

	resp = doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/confirm/"+confirmToken, "", "")
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	resp = doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/subscriptions?email="+testEmail, "", testAPIKey)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.assertSingleSubscription(resp, true)
}

func (s *SubscriptionsSuite) assertSingleSubscription(resp *http.Response, confirmed bool) {
	s.T().Helper()
	var result []map[string]any
	decodeJSON(s.T(), resp, &result)
	s.Require().Len(result, 1)
	s.Equal(testEmail, result[0]["email"])
	s.Equal(testRepoName, result[0]["repo"])
	s.Equal(confirmed, result[0]["confirmed"])
}
