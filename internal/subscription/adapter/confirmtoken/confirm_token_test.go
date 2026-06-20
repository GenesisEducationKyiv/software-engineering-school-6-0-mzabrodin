package confirmtoken_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/subscription/adapter/confirmtoken"
)

const testSecret = "test-secret-key"

type ConfirmTokenSuite struct {
	suite.Suite
}

func TestConfirmTokenSuite(t *testing.T) {
	suite.Run(t, new(ConfirmTokenSuite))
}

func (s *ConfirmTokenSuite) TestIssueAndVerifyRoundTrip() {
	tok := confirmtoken.New(testSecret, time.Hour)

	signed, err := tok.Issue("user@example.com", "owner/repo")
	s.Require().NoError(err)
	s.NotEmpty(signed)

	email, repo, err := tok.Verify(signed)
	s.Require().NoError(err)
	s.Equal("user@example.com", email)
	s.Equal("owner/repo", repo)
}

func (s *ConfirmTokenSuite) TestVerifyExpiredToken() {
	tok := confirmtoken.New(testSecret, -time.Minute)

	signed, err := tok.Issue("user@example.com", "owner/repo")
	s.Require().NoError(err)

	_, _, err = tok.Verify(signed)
	s.ErrorIs(err, confirmtoken.ErrExpired)
}

func (s *ConfirmTokenSuite) TestVerifyMalformedToken() {
	tok := confirmtoken.New(testSecret, time.Hour)

	_, _, err := tok.Verify("not-a-jwt")
	s.ErrorIs(err, confirmtoken.ErrInvalid)
}

func (s *ConfirmTokenSuite) TestVerifyWrongSecret() {
	signed, err := confirmtoken.New(testSecret, time.Hour).Issue("user@example.com", "owner/repo")
	s.Require().NoError(err)

	_, _, err = confirmtoken.New("other-secret", time.Hour).Verify(signed)
	s.ErrorIs(err, confirmtoken.ErrInvalid)
}

func (s *ConfirmTokenSuite) TestVerifyRejectsUnexpectedSigningMethod() {
	claims := jwt.RegisteredClaims{
		Subject:   "user@example.com",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	s.Require().NoError(err)

	_, _, err = confirmtoken.New(testSecret, time.Hour).Verify(signed)
	s.ErrorIs(err, confirmtoken.ErrInvalid)
}
