//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github-release-notifier/internal/subscription/grpc/gen/appv1"
)

type GRPCConfirmSuite struct {
	suite.Suite
	client appv1.SubscriptionServiceClient
}

func TestGRPCConfirmSuite(t *testing.T) {
	suite.Run(t, new(GRPCConfirmSuite))
}

func (s *GRPCConfirmSuite) SetupTest() {
	truncateAll(s.T())
	s.client = newTestGRPCClient(s.T(), true)
}

func (s *GRPCConfirmSuite) TestSuccess() {
	confirmToken, _ := grpcSubscribeAndGetTokens(s.T(), s.client)

	_, err := s.client.Confirm(s.T().Context(), &appv1.ConfirmRequest{Token: confirmToken})
	s.Require().NoError(err)

	var confirmed bool
	row := testPool.QueryRow(s.T().Context(),
		"SELECT confirmed FROM subscriptions WHERE confirm_token=$1", confirmToken)
	s.Require().NoError(row.Scan(&confirmed))
	s.True(confirmed, "subscription should be marked confirmed after Confirm")
}

func (s *GRPCConfirmSuite) TestInvalidTokenLength_InvalidArgument() {
	_, err := s.client.Confirm(s.T().Context(), &appv1.ConfirmRequest{Token: "tooshort"})
	s.Equal(codes.InvalidArgument, status.Code(err))
}

func (s *GRPCConfirmSuite) TestUnknownToken_NotFound() {
	_, err := s.client.Confirm(s.T().Context(), &appv1.ConfirmRequest{Token: randomHex64()})
	s.Equal(codes.NotFound, status.Code(err))
}

func (s *GRPCConfirmSuite) TestAlreadyConfirmed_IsIdempotent() {
	confirmToken, _ := grpcSubscribeAndGetTokens(s.T(), s.client)

	_, err := s.client.Confirm(s.T().Context(), &appv1.ConfirmRequest{Token: confirmToken})
	s.Require().NoError(err)

	_, err = s.client.Confirm(s.T().Context(), &appv1.ConfirmRequest{Token: confirmToken})
	s.NoError(err)
}
