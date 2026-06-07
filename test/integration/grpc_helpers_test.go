//go:build integration

package integration

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	grpcapi "github-release-notifier/internal/adapter/grpc"
	notifierv1 "github-release-notifier/internal/adapter/grpc/gen/notifier/v1"
)

const bufSize = 1024 * 1024

// newTestGRPCConn starts a real gRPC server backed by real repositories against testPool,
// served over an in-memory bufconn listener, and returns a connected client conn.
func newTestGRPCConn(t *testing.T, repoExists bool) *grpc.ClientConn {
	t.Helper()

	uc := newTestUseCases(repoExists)
	handler := grpcapi.NewServer(uc.subscribe, uc.confirm, uc.unsubscribe, uc.list, testLogger)

	srv, err := grpcapi.NewGRPCServer(handler, testAPIKey, testLogger)
	require.NoError(t, err)

	lis := bufconn.Listen(bufSize)
	go func() {
		_ = srv.Serve(lis)
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = conn.Close()
		srv.Stop()
	})

	return conn
}

// newTestGRPCClient returns a SubscriptionService client wired to a fresh test server.
func newTestGRPCClient(t *testing.T, repoExists bool) notifierv1.SubscriptionServiceClient {
	t.Helper()
	return notifierv1.NewSubscriptionServiceClient(newTestGRPCConn(t, repoExists))
}

// grpcAuthCtx attaches the API key as metadata so protected RPCs pass the auth interceptor.
func grpcAuthCtx(ctx context.Context, apiKey string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "x-api-key", apiKey)
}

// grpcSubscribeAndGetTokens subscribes testEmail to testRepoName via gRPC and returns the
// confirmation and unsubscribe tokens by reading them directly from the test database.
func grpcSubscribeAndGetTokens(
	t *testing.T,
	client notifierv1.SubscriptionServiceClient,
) (confirmToken, unsubToken string) {
	t.Helper()

	ctx := grpcAuthCtx(t.Context(), testAPIKey)
	_, err := client.Subscribe(ctx, &notifierv1.SubscribeRequest{Email: testEmail, Repo: testRepoName})
	require.NoError(t, err)

	row := testPool.QueryRow(t.Context(),
		"SELECT confirm_token, unsubscribe_token FROM subscriptions WHERE email=$1", testEmail)
	require.NoError(t, row.Scan(&confirmToken, &unsubToken))

	return
}
