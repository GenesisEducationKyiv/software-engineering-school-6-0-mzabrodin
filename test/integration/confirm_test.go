//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConfirmSuite struct {
	suite.Suite
	srv *httptest.Server
}

func TestConfirmSuite(t *testing.T) {
	suite.Run(t, new(ConfirmSuite))
}

func (s *ConfirmSuite) SetupTest() {
	truncateAll(s.T())
	s.srv = newTestServer(s.T(), true)
}

func (s *ConfirmSuite) TestSuccess() {
	confirmToken, _ := subscribeAndGetTokens(s.T(), s.srv)

	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/confirm/"+confirmToken, "", "")
	s.Equal(http.StatusOK, resp.StatusCode)

	var confirmed bool
	row := testPool.QueryRow(context.Background(),
		"SELECT confirmed FROM subscriptions WHERE confirm_token=$1", confirmToken)
	s.Require().NoError(row.Scan(&confirmed))
	s.True(confirmed, "subscription should be marked confirmed after /api/confirm")
}

func (s *ConfirmSuite) TestInvalidTokenLength_Returns400() {
	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/confirm/tooshort", "", "")
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *ConfirmSuite) TestUnknownToken_Returns404() {
	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/confirm/"+randomHex64(), "", "")
	s.Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *ConfirmSuite) TestAlreadyConfirmed_IsIdempotent() {
	confirmToken, _ := subscribeAndGetTokens(s.T(), s.srv)

	resp := doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/confirm/"+confirmToken, "", "")
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	resp = doRequest(s.T(), http.MethodGet, s.srv.URL+"/api/confirm/"+confirmToken, "", "")
	s.Equal(http.StatusOK, resp.StatusCode)
}
