package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/notifier/domain"
)

type FailedNotificationsRepository struct {
	pool *pgxpool.Pool
}

func NewFailedNotificationsRepository(pool *pgxpool.Pool) *FailedNotificationsRepository {
	return &FailedNotificationsRepository{pool: pool}
}

func (r *FailedNotificationsRepository) Add(ctx context.Context, fn *domain.FailedNotification) error {
	ctx = metrics.WithDBOp(ctx, "add", "failed_notifications")

	if _, err := db.FromContext(ctx, r.pool).Exec(ctx, `
		INSERT INTO failed_notifications (saga_id, repo_name, tag, release_url, email, reason)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (repo_name, tag, email) DO NOTHING
	`, fn.SagaID, fn.RepoName, fn.Tag, fn.ReleaseURL, fn.Email, fn.Reason); err != nil {
		return fmt.Errorf("add failed_notifications: %w", err)
	}

	return nil
}

func (r *FailedNotificationsRepository) ListRetryable(
	ctx context.Context,
	maxRetries int,
) ([]domain.FailedNotification, error) {
	ctx = metrics.WithDBOp(ctx, "list_retryable", "failed_notifications")

	rows, err := r.pool.Query(ctx, `
		SELECT id, saga_id, repo_name, tag, release_url, email, reason, retry_count
		FROM failed_notifications
		WHERE retry_count < $1
		ORDER BY failed_at
	`, maxRetries)
	if err != nil {
		return nil, fmt.Errorf("query failed_notifications: %w", err)
	}

	collectedRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[failedNotificationRow])
	if err != nil {
		return nil, fmt.Errorf("collect failed_notifications: %w", err)
	}

	return toFailedNotifications(collectedRows), nil
}

func (r *FailedNotificationsRepository) IncrementRetry(ctx context.Context, id int64) error {
	ctx = metrics.WithDBOp(ctx, "increment_retry", "failed_notifications")

	if _, err := r.pool.Exec(ctx, `
		UPDATE failed_notifications SET retry_count = retry_count + 1 WHERE id = $1
	`, id); err != nil {
		return fmt.Errorf("increment failed_notifications retry: %w", err)
	}

	return nil
}

func (r *FailedNotificationsRepository) Delete(ctx context.Context, id int64) error {
	ctx = metrics.WithDBOp(ctx, "delete", "failed_notifications")

	if _, err := db.FromContext(ctx, r.pool).Exec(ctx, `
		DELETE FROM failed_notifications WHERE id = $1
	`, id); err != nil {
		return fmt.Errorf("delete failed_notifications: %w", err)
	}

	return nil
}
