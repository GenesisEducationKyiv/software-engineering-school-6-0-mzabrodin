package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github-release-notifier/internal/entity"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GitHubRepoRepository struct {
	pool *pgxpool.Pool
}

func NewGitHubRepoRepository(pool *pgxpool.Pool) *GitHubRepoRepository {
	return &GitHubRepoRepository{pool: pool}
}

func (r *GitHubRepoRepository) Create(ctx context.Context, repo *entity.Repository) (err error) {
	start := time.Now()
	defer func() { trackDBQuery(start, "create", "repositories", err) }()

	err = r.pool.QueryRow(ctx, `
		INSERT INTO repositories (name)
		VALUES ($1)
		RETURNING id, created_at
	`, repo.Name).Scan(&repo.ID, &repo.CreatedAt)

	if err != nil {
		return fmt.Errorf("create repository: %w", err)
	}

	return nil
}

func (r *GitHubRepoRepository) GetByName(ctx context.Context, name string) (result *entity.Repository, err error) {
	start := time.Now()
	defer func() { trackDBQuery(start, "get_by_name", "repositories", err) }()

	repo := &entity.Repository{}
	err = r.pool.QueryRow(ctx, `
		SELECT id, name, last_seen_tag, checked_at, created_at
		FROM repositories WHERE name = $1
	`, name).Scan(&repo.ID, &repo.Name, &repo.LastSeenTag, &repo.CheckedAt, &repo.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, entity.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get repository by name: %w", err)
	}

	return repo, nil
}

func (r *GitHubRepoRepository) GetAllWithSubscriptions(ctx context.Context) (repos []*entity.Repository, err error) {
	start := time.Now()
	defer func() { trackDBQuery(start, "get_all_with_subscriptions", "repositories", err) }()

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

	for rows.Next() {
		repo := &entity.Repository{}
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

func (r *GitHubRepoRepository) UpdateLastSeenTag(ctx context.Context, name, tag string) (err error) {
	start := time.Now()
	defer func() { trackDBQuery(start, "update_last_seen_tag", "repositories", err) }()

	cmd, err := r.pool.Exec(ctx, `
       UPDATE repositories SET last_seen_tag = $1, checked_at = NOW() WHERE name = $2
    `, tag, name)

	if err != nil {
		return fmt.Errorf("update last seen tag: %w", err)
	}

	if cmd.RowsAffected() == 0 {
		return entity.ErrNotFound
	}

	return nil
}
