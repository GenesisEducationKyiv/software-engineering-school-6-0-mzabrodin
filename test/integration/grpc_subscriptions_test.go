//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github-release-notifier/internal/subscription/grpc/gen/subscriptionv1"
)

type GRPCSubscriptionsSuite struct {
	suite.Suite
	client subscriptionv1.SubscriptionServiceClient
}

func TestGRPCSubscriptionsSuite(t *testing.T) {
	suite.Run(t, new(GRPCSubscriptionsSuite))
}

func (s *GRPCSubscriptionsSuite) SetupTest() {
	truncateAll(s.T())
	s.client = newTestGRPCClient(s.T(), true)
}

func (s *GRPCSubscriptionsSuite) list(
	ctx context.Context,
	email string,
) (*subscriptionv1.ListSubscriptionsResponse, error) {
	return s.client.ListSubscriptions(ctx, &subscriptionv1.ListSubscriptionsRequest{Email: email})
}

func (s *GRPCSubscriptionsSuite) TestNoAPIKey_Unauthenticated() {
	_, err := s.list(s.T().Context(), testEmail)
	s.Equal(codes.Unauthenticated, status.Code(err))
}

func (s *GRPCSubscriptionsSuite) TestInvalidEmail_InvalidArgument() {
	ctx := grpcAuthCtx(s.T().Context(), testAPIKey)
	_, err := s.list(ctx, "notanemail")
	s.Equal(codes.InvalidArgument, status.Code(err))
}

func (s *GRPCSubscriptionsSuite) TestEmptyList() {
	ctx := grpcAuthCtx(s.T().Context(), testAPIKey)
	resp, err := s.list(ctx, testEmail)
	s.Require().NoError(err)
	s.Empty(resp.GetSubscriptions())
}

func (s *GRPCSubscriptionsSuite) TestSubscribeConfirmList_EndToEnd() {
	confirmToken, _ := grpcSubscribeAndGetTokens(s.T(), s.client)
	ctx := grpcAuthCtx(s.T().Context(), testAPIKey)

	resp, err := s.list(ctx, testEmail)
	s.Require().NoError(err)
	s.assertSingleSubscription(resp, false)

	_, err = s.client.Confirm(s.T().Context(), &subscriptionv1.ConfirmRequest{Token: confirmToken})
	s.Require().NoError(err)

	resp, err = s.list(ctx, testEmail)
	s.Require().NoError(err)
	s.assertSingleSubscription(resp, true)
}

func (s *GRPCSubscriptionsSuite) assertSingleSubscription(
	resp *subscriptionv1.ListSubscriptionsResponse,
	confirmed bool,
) {
	s.T().Helper()
	subs := resp.GetSubscriptions()
	s.Require().Len(subs, 1)
	s.Equal(testEmail, subs[0].GetEmail())
	s.Equal(testRepoName, subs[0].GetRepo())
	s.Equal(confirmed, subs[0].GetConfirmed())
}
