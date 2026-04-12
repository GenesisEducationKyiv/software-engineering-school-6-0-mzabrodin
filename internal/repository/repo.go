package repository

import (
	"context"
	"errors"
	"fmt"
	"github-release-notifier/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repositoriesRepository struct {
	pool *pgxpool.Pool
}

func NewRepoRepository(pool *pgxpool.Pool) RepoRepository {
	return &repositoriesRepository{pool: pool}
}

func (r *repositoriesRepository) Create(ctx context.Context, repo *domain.Repository) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO repositories (name)
		VALUES ($1)
		RETURNING id, created_at
	`, repo.Name).Scan(&repo.ID, &repo.CreatedAt)

	if err != nil {
		return fmt.Errorf("create repository: %w", err)
	}

	return nil
}

func (r *repositoriesRepository) GetByName(ctx context.Context, name string) (*domain.Repository, error) {
	repo := &domain.Repository{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, last_seen_tag, checked_at, created_at
		FROM repositories WHERE name = $1
	`, name).Scan(&repo.ID, &repo.Name, &repo.LastSeenTag, &repo.CheckedAt, &repo.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get repository by name: %w", err)
	}
	return repo, nil
}

func (r *repositoriesRepository) GetAllWithSubscriptions(ctx context.Context) ([]*domain.Repository, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT r.id, r.name, r.last_seen_tag, r.checked_at, r.created_at
		FROM repositories r
		INNER JOIN subscriptions s ON s.repository_id = r.id
		WHERE s.confirmed = true
	`)
	if err != nil {
		return nil, fmt.Errorf("get all repositories with subscriptions: %w", err)
	}

	defer rows.Close()

	var repos []*domain.Repository
	for rows.Next() {
		repo := &domain.Repository{}
		if err := rows.Scan(&repo.ID, &repo.Name, &repo.LastSeenTag, &repo.CheckedAt, &repo.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan repository: %w", err)
		}

		repos = append(repos, repo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return repos, nil
}

func (r *repositoriesRepository) UpdateLastSeenTag(ctx context.Context, name string, tag string) error {
	cmd, err := r.pool.Exec(ctx, `
       UPDATE repositories SET last_seen_tag = $1, checked_at = NOW() WHERE name = $2
    `, tag, name)

	if err != nil {
		return fmt.Errorf("update last seen tag: %w", err)
	}

	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
