//go:build integration

package integration

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/outbox"
	submigrations "github-release-notifier/internal/subscription/migrations"
)

var (
	testPool     *pgxpool.Pool
	testRedisURL string
	testNATSURL  string
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("test_db"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		slog.Error("start postgres container", "err", err)
		return 1
	}
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			slog.Error("terminate postgres container", "err", err)
		}
	}()

	pgDSN, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		slog.Error("get postgres DSN", "err", err)
		return 1
	}

	integrationLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := db.RunMigrationsFS(pgDSN, submigrations.FS, integrationLogger); err != nil {
		slog.Error("run subscription migrations", "err", err)
		return 1
	}

	if err := db.RunMigrationsFS(pgDSN, outbox.Migrations, integrationLogger,
		db.WithMigrationsTable("outbox_schema_migrations")); err != nil {
		slog.Error("run outbox migrations", "err", err)
		return 1
	}

	testPool, err = db.NewPool(ctx, pgDSN, nil, integrationLogger)
	if err != nil {
		slog.Error("create pool", "err", err)
		return 1
	}
	defer testPool.Close()

	redisContainer, err := tcredis.Run(ctx, "redis:8-alpine")
	if err != nil {
		slog.Error("start redis container", "err", err)
		return 1
	}
	defer func() {
		if err := redisContainer.Terminate(ctx); err != nil {
			slog.Error("terminate redis container", "err", err)
		}
	}()

	testRedisURL, err = redisContainer.ConnectionString(ctx)
	if err != nil {
		slog.Error("get redis connection string", "err", err)
		return 1
	}

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
