//go:build benchmark

package benchmark

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/saga/adapter/compensationclient"
	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/subscription/adapter/compensationserver"
	"github-release-notifier/internal/subscription/usecase/compensate"
)

const (
	benchEmail    = "bench@example.com"
	benchSagaType = "subscribe"
)

type benchCompensator struct{}

func (benchCompensator) Execute(context.Context, compensate.Input) (bool, error) {
	return true, nil
}

func newBenchGRPCClient(b *testing.B) *compensationclient.Client {
	b.Helper()

	svc := compensationserver.NewService(benchCompensator{}, testLogger)

	path, handler, err := compensationserver.NewHandler(svc, testLogger)
	require.NoError(b, err)

	mux := http.NewServeMux()
	mux.Handle(path, handler)

	srv := httptest.NewUnstartedServer(mux)
	srv.Config.Protocols = new(http.Protocols)
	srv.Config.Protocols.SetHTTP1(true)
	srv.Config.Protocols.SetUnencryptedHTTP2(true)
	srv.Start()
	b.Cleanup(srv.Close)

	client, err := compensationclient.Dial(srv.Listener.Addr().String())
	require.NoError(b, err)
	b.Cleanup(func() { _ = client.Close() })

	return client
}

func BenchmarkCompensateGRPC(b *testing.B) {
	client := newBenchGRPCClient(b)
	ctx := context.Background()
	sagaID := uuid.NewString()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := client.Compensate(ctx, sagaID, benchSagaType, benchEmail, testRepoName); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompensateNATS(b *testing.B) {
	ctx := context.Background()

	conn, err := broker.Connect(testNATSURL, testLogger)
	require.NoError(b, err)
	b.Cleanup(func() { _ = conn.Close() })

	id := strings.ReplaceAll(uuid.NewString(), "-", "")
	stream := "BENCH_" + id
	subject := "bench." + id
	require.NoError(b, conn.EnsureStream(ctx, stream, []string{subject}))

	processed := make(chan struct{}, 1)
	stop, err := conn.Consume(ctx, stream, "bench-consumer", subject, func(context.Context, []byte) error {
		processed <- struct{}{}

		return nil
	})
	require.NoError(b, err)
	b.Cleanup(stop)

	data, err := events.Marshal(events.SagaCompensate{
		SagaID:   uuid.NewString(),
		SagaType: benchSagaType,
		Email:    benchEmail,
		RepoName: testRepoName,
	})
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		require.NoError(b, conn.Publish(ctx, subject, data))
		<-processed
	}

	b.StopTimer()
}
