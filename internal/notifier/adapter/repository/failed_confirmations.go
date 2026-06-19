package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/notifier/domain"
)

type FailedConfirmationsRepository struct {
	pool *pgxpool.Pool
}

func NewFailedConfirmationsRepository(pool *pgxpool.Pool) *FailedConfirmationsRepository {
	return &FailedConfirmationsRepository{pool: pool}
}

func (r *FailedConfirmationsRepository) Add(ctx context.Context, fc *domain.FailedConfirmation) error {
	ctx = metrics.WithDBOp(ctx, "add", "failed_confirmations")

	if _, err := r.pool.Exec(ctx, `
		INSERT INTO failed_confirmations (saga_id, email, repo_name, confirm_url, reason)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email, repo_name) DO NOTHING
	`, fc.SagaID, fc.Email, fc.RepoName, fc.ConfirmURL, fc.Reason); err != nil {
		return fmt.Errorf("add failed_confirmations: %w", err)
	}

	return nil
}

func (r *FailedConfirmationsRepository) ListRetryable(
	ctx context.Context,
	maxRetries int,
	notBefore time.Time,
) ([]domain.FailedConfirmation, error) {
	ctx = metrics.WithDBOp(ctx, "list_retryable", "failed_confirmations")

	rows, err := r.pool.Query(ctx, `
		SELECT id, saga_id, email, repo_name, confirm_url, reason, retry_count
		FROM failed_confirmations
		WHERE retry_count < $1 AND failed_at > $2
		ORDER BY failed_at
	`, maxRetries, notBefore)
	if err != nil {
		return nil, fmt.Errorf("query failed_confirmations: %w", err)
	}

	collectedRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[failedConfirmationRow])
	if err != nil {
		return nil, fmt.Errorf("collect failed_confirmations: %w", err)
	}

	return toFailedConfirmations(collectedRows), nil
}

func (r *FailedConfirmationsRepository) ListExpired(
	ctx context.Context,
	cutoff time.Time,
) ([]domain.FailedConfirmation, error) {
	ctx = metrics.WithDBOp(ctx, "list_expired", "failed_confirmations")

	rows, err := r.pool.Query(ctx, `
		SELECT id, saga_id, email, repo_name, confirm_url, reason, retry_count
		FROM failed_confirmations
		WHERE failed_at <= $1
		ORDER BY failed_at
	`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("query expired failed_confirmations: %w", err)
	}

	collectedRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[failedConfirmationRow])
	if err != nil {
		return nil, fmt.Errorf("collect expired failed_confirmations: %w", err)
	}

	return toFailedConfirmations(collectedRows), nil
}

func (r *FailedConfirmationsRepository) IncrementRetry(ctx context.Context, id int64) error {
	ctx = metrics.WithDBOp(ctx, "increment_retry", "failed_confirmations")

	if _, err := r.pool.Exec(ctx, `
		UPDATE failed_confirmations SET retry_count = retry_count + 1 WHERE id = $1
	`, id); err != nil {
		return fmt.Errorf("increment failed_confirmations retry: %w", err)
	}

	return nil
}

func (r *FailedConfirmationsRepository) Delete(ctx context.Context, id int64) error {
	ctx = metrics.WithDBOp(ctx, "delete", "failed_confirmations")

	if _, err := r.pool.Exec(ctx, `
		DELETE FROM failed_confirmations WHERE id = $1
	`, id); err != nil {
		return fmt.Errorf("delete failed_confirmations: %w", err)
	}

	return nil
}
