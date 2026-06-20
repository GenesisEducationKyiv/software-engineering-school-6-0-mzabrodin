//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github-release-notifier/internal/subscription/grpc/gen/subscriptionv1"
)

func newTestGRPCConn(t *testing.T, repoExists bool) *grpc.ClientConn {
	t.Helper()

	srv := newTestServer(t, repoExists)

	conn, err := grpc.NewClient(
		srv.Listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = conn.Close()
	})

	return conn
}

func newTestGRPCClient(t *testing.T, repoExists bool) subscriptionv1.SubscriptionServiceClient {
	t.Helper()
	return subscriptionv1.NewSubscriptionServiceClient(newTestGRPCConn(t, repoExists))
}

func grpcAuthCtx(ctx context.Context, apiKey string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+apiKey)
}

func grpcSubscribeAndGetTokens(
	t *testing.T,
	client subscriptionv1.SubscriptionServiceClient,
) (confirmToken, unsubToken string) {
	t.Helper()

	ctx := grpcAuthCtx(t.Context(), testAPIKey)
	_, err := client.Subscribe(ctx, &subscriptionv1.SubscribeRequest{Email: testEmail, Repo: testRepoName})
	require.NoError(t, err)

	row := testPool.QueryRow(t.Context(),
		"SELECT unsubscribe_token FROM subscriptions WHERE email=$1", testEmail)
	require.NoError(t, row.Scan(&unsubToken))

	return confirmTokenFor(t, testEmail), unsubToken
}
