package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/shared/entity"
)

type GitHubRepoRepository struct {
	pool *pgxpool.Pool
}

func NewGitHubRepoRepository(pool *pgxpool.Pool) *GitHubRepoRepository {
	return &GitHubRepoRepository{pool: pool}
}

func (r *GitHubRepoRepository) GetOrCreate(ctx context.Context, name string) (entity.Repository, error) {
	ctx = metrics.WithDBOp(ctx, "get_or_create", "repositories")

	rows, err := db.FromContext(ctx, r.pool).Query(ctx, `
		INSERT INTO repositories (name)
		VALUES ($1)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, name, created_at
	`, name)
	if err != nil {
		return entity.Repository{}, fmt.Errorf("get or create repository: %w", err)
	}

	collectedRow, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[repositoryRow])
	if err != nil {
		return entity.Repository{}, fmt.Errorf("get or create repository: %w", err)
	}

	return collectedRow.toEntity(), nil
}
