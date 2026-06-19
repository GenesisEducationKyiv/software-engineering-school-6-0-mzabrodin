package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string, tracer pgx.QueryTracer, log *slog.Logger) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}

	config.ConnConfig.Tracer = tracer

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	log.Info("database connected successfully")
	return pool, nil
}

type MigrateOption func(*migrateConfig)

type migrateConfig struct {
	migrationsTable string
}

func WithMigrationsTable(name string) MigrateOption {
	return func(c *migrateConfig) { c.migrationsTable = name }
}

func RunMigrationsFS(databaseURL string, fsys fs.FS, log *slog.Logger, opts ...MigrateOption) error {
	var cfg migrateConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	src, err := iofs.New(fsys, migrationsRoot(fsys))
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}

	migrator, err := migrate.NewWithSourceInstance("iofs", src, withMigrationsTable(databaseURL, cfg.migrationsTable))
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	defer func(m *migrate.Migrate) {
		sourceErr, databaseErr := m.Close()
		if sourceErr != nil {
			log.Error("failed to close migration source", "error", sourceErr)
		}

		if databaseErr != nil {
			log.Error("failed to close migration database", "error", databaseErr)
		}
	}(migrator)

	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	log.Info("migrations applied successfully")

	return nil
}

func migrationsRoot(fsys fs.FS) string {
	if _, err := fs.Stat(fsys, "migrations"); err == nil {
		return "migrations"
	}

	return "."
}

func withMigrationsTable(databaseURL, table string) string {
	if table == "" {
		return databaseURL
	}

	u, err := url.Parse(databaseURL)
	if err != nil {
		return databaseURL
	}

	q := u.Query()
	q.Set("x-migrations-table", table)
	u.RawQuery = q.Encode()

	return u.String()
}
