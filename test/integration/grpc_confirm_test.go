//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github-release-notifier/internal/subscription/grpc/gen/subscriptionv1"
)

type GRPCConfirmSuite struct {
	suite.Suite
	client subscriptionv1.SubscriptionServiceClient
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

	_, err := s.client.Confirm(s.T().Context(), &subscriptionv1.ConfirmRequest{Token: confirmToken})
	s.Require().NoError(err)

	var confirmed bool
	row := testPool.QueryRow(s.T().Context(),
		"SELECT confirmed FROM subscriptions WHERE email=$1", testEmail)
	s.Require().NoError(row.Scan(&confirmed))
	s.True(confirmed, "subscription should be marked confirmed after Confirm")
}

func (s *GRPCConfirmSuite) TestMalformedToken_InvalidArgument() {
	_, err := s.client.Confirm(s.T().Context(), &subscriptionv1.ConfirmRequest{Token: "not-a-jwt"})
	s.Equal(codes.InvalidArgument, status.Code(err))
}

func (s *GRPCConfirmSuite) TestUnknownSubscriber_IsIdempotent() {
	token := confirmTokenFor(s.T(), "stranger@example.com")

	_, err := s.client.Confirm(s.T().Context(), &subscriptionv1.ConfirmRequest{Token: token})
	s.NoError(err)
}

func (s *GRPCConfirmSuite) TestAlreadyConfirmed_IsIdempotent() {
	confirmToken, _ := grpcSubscribeAndGetTokens(s.T(), s.client)

	_, err := s.client.Confirm(s.T().Context(), &subscriptionv1.ConfirmRequest{Token: confirmToken})
	s.Require().NoError(err)

	_, err = s.client.Confirm(s.T().Context(), &subscriptionv1.ConfirmRequest{Token: confirmToken})
	s.NoError(err)
}
