package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxRetries    = 10
	retryInterval = 2 * time.Second
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	var (
		pool *pgxpool.Pool
		err  error
	)

	for i := range maxRetries {
		pool, err = pgxpool.New(ctx, databaseURL)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				log.Println("database connected successfully")
				return pool, nil
			} else {
				pool.Close()
				err = pingErr
			}
		}

		log.Printf("waiting for database. attempt %d/%d: %v", i+1, maxRetries, err)
		time.Sleep(retryInterval)
	}

	return nil, fmt.Errorf("database is not ready after %d attempts: %w", maxRetries, err)
}

func RunMigrations(databaseURL string) error {
	migrator, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	defer func(m *migrate.Migrate) {
		sourceErr, databaseErr := m.Close()
		if sourceErr != nil {
			log.Printf("failed to close migration source: %v", sourceErr)
		}

		if databaseErr != nil {
			log.Printf("failed to close migration database: %v", databaseErr)
		}
	}(migrator)

	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	log.Println("migrations applied successfully")
	return nil
}
