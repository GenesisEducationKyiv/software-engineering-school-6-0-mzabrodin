package repository

import (
	"context"
	"errors"
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

func (r *GitHubRepoRepository) Create(ctx context.Context, repo entity.Repository) (entity.Repository, error) {
	ctx = metrics.WithDBOp(ctx, "create", "repositories")

	if err := db.FromContext(ctx, r.pool).QueryRow(ctx, `
		INSERT INTO repositories (name)
		VALUES ($1)
		RETURNING id, created_at
	`, repo.Name).Scan(&repo.ID, &repo.CreatedAt); err != nil {
		return entity.Repository{}, fmt.Errorf("create repository: %w", err)
	}

	return repo, nil
}

func (r *GitHubRepoRepository) GetByName(ctx context.Context, name string) (entity.Repository, error) {
	ctx = metrics.WithDBOp(ctx, "get_by_name", "repositories")

	rows, err := db.FromContext(ctx, r.pool).Query(ctx, `
		SELECT id, name, created_at
		FROM repositories WHERE name = $1
	`, name)
	if err != nil {
		return entity.Repository{}, fmt.Errorf("get repository by name: %w", err)
	}

	collectedRow, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[repositoryRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Repository{}, entity.ErrNotFound
	}

	if err != nil {
		return entity.Repository{}, fmt.Errorf("get repository by name: %w", err)
	}

	return collectedRow.toEntity(), nil
}
