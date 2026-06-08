//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	notifierv1 "github-release-notifier/internal/adapter/grpc/gen/app/v1"
)

type GRPCSubscribeSuite struct {
	suite.Suite
	client notifierv1.SubscriptionServiceClient
}

func TestGRPCSubscribeSuite(t *testing.T) {
	suite.Run(t, new(GRPCSubscribeSuite))
}

func (s *GRPCSubscribeSuite) SetupTest() {
	truncateAll(s.T())
	s.client = newTestGRPCClient(s.T(), true)
}

func (s *GRPCSubscribeSuite) TestSuccess() {
	ctx := grpcAuthCtx(s.T().Context(), testAPIKey)
	_, err := s.client.Subscribe(ctx, &notifierv1.SubscribeRequest{Email: testEmail, Repo: testRepoName})
	s.Require().NoError(err)

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

func (s *GRPCSubscribeSuite) TestNoAPIKey_Unauthenticated() {
	_, err := s.client.Subscribe(s.T().Context(),
		&notifierv1.SubscribeRequest{Email: testEmail, Repo: testRepoName})
	s.Equal(codes.Unauthenticated, status.Code(err))
}

func (s *GRPCSubscribeSuite) TestWrongAPIKey_Unauthenticated() {
	ctx := grpcAuthCtx(s.T().Context(), "wrong-key")
	_, err := s.client.Subscribe(ctx, &notifierv1.SubscribeRequest{Email: testEmail, Repo: testRepoName})
	s.Equal(codes.Unauthenticated, status.Code(err))
}

func (s *GRPCSubscribeSuite) TestEmptyEmail_InvalidArgument() {
	ctx := grpcAuthCtx(s.T().Context(), testAPIKey)
	_, err := s.client.Subscribe(ctx, &notifierv1.SubscribeRequest{Email: "", Repo: testRepoName})
	s.Equal(codes.InvalidArgument, status.Code(err))
}

func (s *GRPCSubscribeSuite) TestInvalidEmail_InvalidArgument() {
	ctx := grpcAuthCtx(s.T().Context(), testAPIKey)
	_, err := s.client.Subscribe(ctx, &notifierv1.SubscribeRequest{Email: "notanemail", Repo: testRepoName})
	s.Equal(codes.InvalidArgument, status.Code(err))
}

func (s *GRPCSubscribeSuite) TestEmptyRepo_InvalidArgument() {
	ctx := grpcAuthCtx(s.T().Context(), testAPIKey)
	_, err := s.client.Subscribe(ctx, &notifierv1.SubscribeRequest{Email: testEmail, Repo: ""})
	s.Equal(codes.InvalidArgument, status.Code(err))
}

func (s *GRPCSubscribeSuite) TestInvalidRepo_InvalidArgument() {
	ctx := grpcAuthCtx(s.T().Context(), testAPIKey)
	_, err := s.client.Subscribe(ctx, &notifierv1.SubscribeRequest{Email: testEmail, Repo: "noslash"})
	s.Equal(codes.InvalidArgument, status.Code(err))
}

func (s *GRPCSubscribeSuite) TestRepoNotOnGitHub_NotFound() {
	client := newTestGRPCClient(s.T(), false)
	ctx := grpcAuthCtx(s.T().Context(), testAPIKey)
	_, err := client.Subscribe(ctx, &notifierv1.SubscribeRequest{Email: testEmail, Repo: testRepoName})
	s.Equal(codes.NotFound, status.Code(err))
}

func (s *GRPCSubscribeSuite) TestDuplicate_AlreadyExists() {
	ctx := grpcAuthCtx(s.T().Context(), testAPIKey)
	req := &notifierv1.SubscribeRequest{Email: testEmail, Repo: testRepoName}

	_, err := s.client.Subscribe(ctx, req)
	s.Require().NoError(err)

	_, err = s.client.Subscribe(ctx, req)
	s.Equal(codes.AlreadyExists, status.Code(err))
}
