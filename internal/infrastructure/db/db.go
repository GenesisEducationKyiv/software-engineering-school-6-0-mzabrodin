package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string, log *slog.Logger) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
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

func RunMigrations(databaseURL, migrationsURL string, log *slog.Logger) error {
	migrator, err := migrate.New(migrationsURL, databaseURL)
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
