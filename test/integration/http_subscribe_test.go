//go:build integration

package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type HTTPSubscribeSuite struct {
	suite.Suite
	srv *httptest.Server
}

func TestHTTPSubscribeSuite(t *testing.T) {
	suite.Run(t, new(HTTPSubscribeSuite))
}

func (s *HTTPSubscribeSuite) SetupTest() {
	truncateAll(s.T())
	s.srv = newTestHTTPServer(s.T(), true)
}

func (s *HTTPSubscribeSuite) TestSuccess() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, testAPIKey)
	s.Equal(http.StatusOK, resp.StatusCode)

	var email, repo string
	var confirmed bool
	row := testPool.QueryRow(s.T().Context(), `
		SELECT s.email, r.name, s.confirmed
		FROM subscriptions s
		JOIN repositories r ON r.id = s.repository_id
		WHERE s.email = $1`, testEmail)
	s.Require().NoError(row.Scan(&email, &repo, &confirmed))
	s.Equal(testEmail, email)
	s.Equal(testRepoName, repo)
	s.False(confirmed, "new subscription must not be confirmed")
}

func (s *HTTPSubscribeSuite) TestNoAPIKey_Returns401() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, "")
	s.Equal(http.StatusUnauthorized, resp.StatusCode)
}

func (s *HTTPSubscribeSuite) TestWrongAPIKey_Returns401() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, "wrong-key")
	s.Equal(http.StatusUnauthorized, resp.StatusCode)
}

func (s *HTTPSubscribeSuite) TestMalformedJSON_Returns500() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe", "not-json", testAPIKey)
	s.Equal(http.StatusInternalServerError, resp.StatusCode)
}

func (s *HTTPSubscribeSuite) TestEmptyEmail_Returns400() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe",
		`{"email":"","repo":"owner/repo"}`, testAPIKey)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *HTTPSubscribeSuite) TestEmptyRepo_Returns400() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":""}`, testAPIKey)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *HTTPSubscribeSuite) TestInvalidEmail_Returns400() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe",
		`{"email":"notanemail","repo":"owner/repo"}`, testAPIKey)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *HTTPSubscribeSuite) TestInvalidRepo_Returns400() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"noslash"}`, testAPIKey)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *HTTPSubscribeSuite) TestRepoNotOnGitHub_Returns404() {
	srv := newTestHTTPServer(s.T(), false)
	resp := doRequest(s.T(), http.MethodPost, srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, testAPIKey)
	s.Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *HTTPSubscribeSuite) TestDuplicate_Returns409() {
	body := `{"email":"user@example.com","repo":"owner/repo"}`
	doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe", body, testAPIKey)

	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe", body, testAPIKey)
	s.Equal(http.StatusConflict, resp.StatusCode)
}
