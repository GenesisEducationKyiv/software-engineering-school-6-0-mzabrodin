package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/infrastructure/metrics"
)

type ProcessedReleasesRepository struct {
	pool *pgxpool.Pool
}

func NewProcessedReleasesRepository(pool *pgxpool.Pool) *ProcessedReleasesRepository {
	return &ProcessedReleasesRepository{pool: pool}
}

func (r *ProcessedReleasesRepository) Exists(ctx context.Context, repoName, tag string) (bool, error) {
	ctx = metrics.WithDBOp(ctx, "exists", "processed_releases")

	var exists bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM processed_releases WHERE repo_name = $1 AND tag = $2)
	`, repoName, tag).Scan(&exists); err != nil {
		return false, fmt.Errorf("check processed_releases: %w", err)
	}

	return exists, nil
}

func (r *ProcessedReleasesRepository) Mark(ctx context.Context, repoName, tag string) error {
	ctx = metrics.WithDBOp(ctx, "mark", "processed_releases")

	if _, err := r.pool.Exec(ctx, `
		INSERT INTO processed_releases (repo_name, tag) VALUES ($1, $2)
		ON CONFLICT (repo_name, tag) DO NOTHING
	`, repoName, tag); err != nil {
		return fmt.Errorf("mark processed_releases: %w", err)
	}

	return nil
}

func (r *ProcessedReleasesRepository) DeleteProcessedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	ctx = metrics.WithDBOp(ctx, "delete_processed_before", "processed_releases")

	commandTag, err := r.pool.Exec(ctx, `
		DELETE FROM processed_releases WHERE processed_at < $1
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old processed_releases: %w", err)
	}

	return commandTag.RowsAffected(), nil
}
