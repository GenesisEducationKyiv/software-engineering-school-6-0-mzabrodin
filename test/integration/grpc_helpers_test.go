//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github-release-notifier/internal/subscription/grpc/gen/appv1"
)

// newTestGRPCConn dials the unified public handler (served over h2c by
// newTestServer) with a plain gRPC client, exercising the gRPC protocol through
// Vanguard end to end.
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

// newTestGRPCClient returns a SubscriptionService client wired to a fresh test server.
func newTestGRPCClient(t *testing.T, repoExists bool) appv1.SubscriptionServiceClient {
	t.Helper()
	return appv1.NewSubscriptionServiceClient(newTestGRPCConn(t, repoExists))
}

// grpcAuthCtx attaches the API key as Authorization metadata so protected RPCs pass the auth interceptor.
func grpcAuthCtx(ctx context.Context, apiKey string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+apiKey)
}

// grpcSubscribeAndGetTokens subscribes testEmail to testRepoName via gRPC and returns the
// confirmation and unsubscribe tokens by reading them directly from the test database.
func grpcSubscribeAndGetTokens(
	t *testing.T,
	client appv1.SubscriptionServiceClient,
) (confirmToken, unsubToken string) {
	t.Helper()

	ctx := grpcAuthCtx(t.Context(), testAPIKey)
	_, err := client.Subscribe(ctx, &appv1.SubscribeRequest{Email: testEmail, Repo: testRepoName})
	require.NoError(t, err)

	row := testPool.QueryRow(t.Context(),
		"SELECT confirm_token, unsubscribe_token FROM subscriptions WHERE email=$1", testEmail)
	require.NoError(t, row.Scan(&confirmToken, &unsubToken))

	return
}
