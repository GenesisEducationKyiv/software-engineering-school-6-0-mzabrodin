//go:build integration

package integration

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github-release-notifier/internal/db"
)

var (
	testPool     *pgxpool.Pool
	testRedisURL string
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

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		slog.Error("cannot determine source file path")
		return 1
	}

	migrationsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")
	migrationsURL := "file://" + filepath.ToSlash(migrationsDir)

	integrationLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := db.RunMigrations(pgDSN, migrationsURL, integrationLogger); err != nil {
		slog.Error("run migrations", "err", err)
		return 1
	}

	testPool, err = db.NewPool(ctx, pgDSN, integrationLogger)
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

	return m.Run()
}
