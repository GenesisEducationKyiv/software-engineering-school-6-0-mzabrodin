//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SubscribeSuite struct {
	suite.Suite
	srv *httptest.Server
}

func TestSubscribeSuite(t *testing.T) {
	suite.Run(t, new(SubscribeSuite))
}

func (s *SubscribeSuite) SetupTest() {
	truncateAll(s.T())
	s.srv = newTestServer(s.T(), true)
}

func (s *SubscribeSuite) TestSuccess() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, testAPIKey)
	s.Equal(http.StatusOK, resp.StatusCode)

	var email, repo string
	var confirmed bool
	row := testPool.QueryRow(context.Background(), `
		SELECT s.email, r.name, s.confirmed
		FROM subscriptions s
		JOIN repositories r ON r.id = s.repository_id
		WHERE s.email = $1`, testEmail)
	s.Require().NoError(row.Scan(&email, &repo, &confirmed))
	s.Equal(testEmail, email)
	s.Equal(testRepoName, repo)
	s.False(confirmed, "new subscription must not be confirmed")
}

func (s *SubscribeSuite) TestNoAPIKey_Returns401() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, "")
	s.Equal(http.StatusUnauthorized, resp.StatusCode)
}

func (s *SubscribeSuite) TestWrongAPIKey_Returns401() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, "wrong-key")
	s.Equal(http.StatusUnauthorized, resp.StatusCode)
}

func (s *SubscribeSuite) TestInvalidJSON_Returns400() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe", "not-json", testAPIKey)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *SubscribeSuite) TestEmptyEmail_Returns400() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe",
		`{"email":"","repo":"owner/repo"}`, testAPIKey)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *SubscribeSuite) TestEmptyRepo_Returns400() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":""}`, testAPIKey)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *SubscribeSuite) TestInvalidEmail_Returns400() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe",
		`{"email":"notanemail","repo":"owner/repo"}`, testAPIKey)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *SubscribeSuite) TestInvalidRepo_Returns400() {
	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"noslash"}`, testAPIKey)
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *SubscribeSuite) TestRepoNotOnGitHub_Returns404() {
	srv := newTestServer(s.T(), false)
	resp := doRequest(s.T(), http.MethodPost, srv.URL+"/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, testAPIKey)
	s.Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *SubscribeSuite) TestDuplicate_Returns409() {
	body := `{"email":"user@example.com","repo":"owner/repo"}`
	doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe", body, testAPIKey)

	resp := doRequest(s.T(), http.MethodPost, s.srv.URL+"/api/subscribe", body, testAPIKey)
	s.Equal(http.StatusConflict, resp.StatusCode)
}
