package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/shared/entity"
)

type GitHubRepoRepository struct {
	pool *pgxpool.Pool
}

func NewGitHubRepoRepository(pool *pgxpool.Pool) *GitHubRepoRepository {
	return &GitHubRepoRepository{pool: pool}
}

func (r *GitHubRepoRepository) Create(ctx context.Context, repo *entity.Repository) error {
	ctx = metrics.WithDBOp(ctx, "create", "repositories")

	if err := r.pool.QueryRow(ctx, `
		INSERT INTO repositories (name)
		VALUES ($1)
		RETURNING id, created_at
	`, repo.Name).Scan(&repo.ID, &repo.CreatedAt); err != nil {
		return fmt.Errorf("create repository: %w", err)
	}

	return nil
}

func (r *GitHubRepoRepository) GetByName(ctx context.Context, name string) (*entity.Repository, error) {
	ctx = metrics.WithDBOp(ctx, "get_by_name", "repositories")

	rows, err := r.pool.Query(ctx, `
		SELECT id, name, last_seen_tag, checked_at, created_at
		FROM repositories WHERE name = $1
	`, name)
	if err != nil {
		return nil, fmt.Errorf("get repository by name: %w", err)
	}

	collectedRow, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[repositoryRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, entity.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get repository by name: %w", err)
	}

	return collectedRow.toEntity(), nil
}

func (r *GitHubRepoRepository) GetAllWithSubscriptions(ctx context.Context) ([]*entity.Repository, error) {
	ctx = metrics.WithDBOp(ctx, "get_all_with_subscriptions", "repositories")

	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT r.id, r.name, r.last_seen_tag, r.checked_at, r.created_at
		FROM repositories r
		INNER JOIN subscriptions s ON s.repository_id = r.id
		WHERE s.confirmed = true
	`)
	if err != nil {
		return nil, fmt.Errorf("get all repositories with subscriptions: %w", err)
	}

	collectedRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[repositoryRow])
	if err != nil {
		return nil, fmt.Errorf("collect repositories: %w", err)
	}

	return toRepositoryEntities(collectedRows), nil
}

func (r *GitHubRepoRepository) UpdateLastSeenTag(ctx context.Context, name, tag string) error {
	ctx = metrics.WithDBOp(ctx, "update_last_seen_tag", "repositories")

	commandTag, err := r.pool.Exec(ctx, `
		UPDATE repositories SET last_seen_tag = $1, checked_at = NOW() WHERE name = $2
	`, tag, name)
	if err != nil {
		return fmt.Errorf("update last seen tag: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return entity.ErrNotFound
	}

	return nil
}
