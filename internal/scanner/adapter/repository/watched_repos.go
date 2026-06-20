package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/scanner/domain"
)

type WatchedRepoRepository struct {
	pool *pgxpool.Pool
}

func NewWatchedRepoRepository(pool *pgxpool.Pool) *WatchedRepoRepository {
	return &WatchedRepoRepository{pool: pool}
}

func (r *WatchedRepoRepository) IncrementSubscriber(ctx context.Context, repoName string) error {
	ctx = metrics.WithDBOp(ctx, "increment_subscriber", "watched_repos")

	if _, err := r.pool.Exec(ctx, `
		INSERT INTO watched_repos (repo_name, subscriber_count)
		VALUES ($1, 1)
		ON CONFLICT (repo_name) DO UPDATE SET subscriber_count = watched_repos.subscriber_count + 1
	`, repoName); err != nil {
		return fmt.Errorf("increment subscriber count: %w", err)
	}

	return nil
}

func (r *WatchedRepoRepository) DecrementSubscriber(ctx context.Context, repoName string) error {
	ctx = metrics.WithDBOp(ctx, "decrement_subscriber", "watched_repos")

	if _, err := r.pool.Exec(ctx, `
		WITH decremented AS (
			UPDATE watched_repos
			SET subscriber_count = subscriber_count - 1
			WHERE repo_name = $1 AND subscriber_count > 1
			RETURNING repo_name
		)
		DELETE FROM watched_repos
		WHERE repo_name = $1 AND NOT EXISTS (SELECT 1 FROM decremented)
	`, repoName); err != nil {
		return fmt.Errorf("decrement subscriber count: %w", err)
	}

	return nil
}

func (r *WatchedRepoRepository) ListWatched(ctx context.Context) ([]domain.WatchedRepo, error) {
	ctx = metrics.WithDBOp(ctx, "list_watched", "watched_repos")

	rows, err := r.pool.Query(ctx, `
		SELECT repo_name, last_seen_tag, subscriber_count
		FROM watched_repos
		WHERE subscriber_count > 0
	`)
	if err != nil {
		return nil, fmt.Errorf("query watched repos: %w", err)
	}

	collectedRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[watchedRepoRow])
	if err != nil {
		return nil, fmt.Errorf("collect watched repos: %w", err)
	}

	return toWatchedRepos(collectedRows), nil
}

func (r *WatchedRepoRepository) AdvanceTag(ctx context.Context, repoName, tag string) error {
	ctx = metrics.WithDBOp(ctx, "advance_tag", "watched_repos")

	if _, err := r.pool.Exec(ctx, `
		UPDATE watched_repos SET last_seen_tag = $2 WHERE repo_name = $1
	`, repoName, tag); err != nil {
		return fmt.Errorf("advance last seen tag: %w", err)
	}

	return nil
}
