//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	notifierv1 "github-release-notifier/internal/adapter/grpc/gen/notifier/v1"
)

type GRPCUnsubscribeSuite struct {
	suite.Suite
	client notifierv1.SubscriptionServiceClient
}

func TestGRPCUnsubscribeSuite(t *testing.T) {
	suite.Run(t, new(GRPCUnsubscribeSuite))
}

func (s *GRPCUnsubscribeSuite) SetupTest() {
	truncateAll(s.T())
	s.client = newTestGRPCClient(s.T(), true)
}

func (s *GRPCUnsubscribeSuite) TestSuccess() {
	_, unsubToken := grpcSubscribeAndGetTokens(s.T(), s.client)

	_, err := s.client.Unsubscribe(s.T().Context(), &notifierv1.UnsubscribeRequest{Token: unsubToken})
	s.Require().NoError(err)

	var count int
	row := testPool.QueryRow(s.T().Context(),
		"SELECT COUNT(*) FROM subscriptions WHERE unsubscribe_token=$1", unsubToken)
	s.Require().NoError(row.Scan(&count))
	s.Zero(count, "subscription should be deleted after Unsubscribe")
}

func (s *GRPCUnsubscribeSuite) TestInvalidTokenLength_InvalidArgument() {
	_, err := s.client.Unsubscribe(s.T().Context(), &notifierv1.UnsubscribeRequest{Token: "tooshort"})
	s.Equal(codes.InvalidArgument, status.Code(err))
}

func (s *GRPCUnsubscribeSuite) TestUnknownToken_NotFound() {
	_, err := s.client.Unsubscribe(s.T().Context(), &notifierv1.UnsubscribeRequest{Token: randomHex64()})
	s.Equal(codes.NotFound, status.Code(err))
}

func (s *GRPCUnsubscribeSuite) TestAlreadyUnsubscribed_NotFound() {
	_, unsubToken := grpcSubscribeAndGetTokens(s.T(), s.client)

	_, err := s.client.Unsubscribe(s.T().Context(), &notifierv1.UnsubscribeRequest{Token: unsubToken})
	s.Require().NoError(err)

	_, err = s.client.Unsubscribe(s.T().Context(), &notifierv1.UnsubscribeRequest{Token: unsubToken})
	s.Equal(codes.NotFound, status.Code(err))
}
