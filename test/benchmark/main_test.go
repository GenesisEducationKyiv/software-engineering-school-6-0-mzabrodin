//go:build benchmark

package benchmark

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
)

const testRepoName = "owner/repo"

var (
	testLogger  = slog.New(slog.NewTextHandler(io.Discard, nil))
	testNATSURL string
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	natsContainer, err := tcnats.Run(ctx, "nats:2-alpine")
	if err != nil {
		slog.Error("start nats container", "err", err)

		return 1
	}
	defer func() {
		if err := natsContainer.Terminate(ctx); err != nil {
			slog.Error("terminate nats container", "err", err)
		}
	}()

	testNATSURL, err = natsContainer.ConnectionString(ctx)
	if err != nil {
		slog.Error("get nats connection string", "err", err)

		return 1
	}

	return m.Run()
}
