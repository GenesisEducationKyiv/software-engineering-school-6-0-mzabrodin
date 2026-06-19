package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/subscription/domain"
)

type SubscriptionRepository struct {
	pool *pgxpool.Pool
}

func NewSubscriptionRepository(pool *pgxpool.Pool) *SubscriptionRepository {
	return &SubscriptionRepository{pool: pool}
}

func (r *SubscriptionRepository) Create(ctx context.Context, sub *entity.Subscription) error {
	ctx = metrics.WithDBOp(ctx, "create", "subscriptions")

	err := r.pool.QueryRow(ctx, `
		INSERT INTO subscriptions (repository_id, email, confirm_token, unsubscribe_token, confirmed)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email, repository_id) DO NOTHING
		RETURNING id, created_at
	`, sub.RepositoryID, sub.Email, sub.ConfirmToken, sub.UnsubscribeToken, sub.Confirmed).Scan(&sub.ID, &sub.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.ErrAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}

	return nil
}

func (r *SubscriptionRepository) GetByEmail(
	ctx context.Context,
	email string,
) ([]*domain.SubscriptionView, error) {
	ctx = metrics.WithDBOp(ctx, "get_by_email", "subscriptions")

	rows, err := r.pool.Query(ctx, `
		SELECT s.email, r.name AS repo, s.confirmed, r.last_seen_tag
		FROM subscriptions s
		JOIN repositories r ON r.id = s.repository_id
		WHERE s.email = $1
		ORDER BY s.created_at DESC
	`, email)
	if err != nil {
		return nil, fmt.Errorf("get by email: %w", err)
	}

	collectedRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[subscriptionViewRow])
	if err != nil {
		return nil, fmt.Errorf("collect subscription views: %w", err)
	}

	return toSubscriptionViewEntities(collectedRows), nil
}

func (r *SubscriptionRepository) GetConfirmedByRepoID(
	ctx context.Context,
	repoID uuid.UUID,
) ([]*entity.Subscription, error) {
	ctx = metrics.WithDBOp(ctx, "get_confirmed_by_repo_id", "subscriptions")

	rows, err := r.pool.Query(ctx, `
		SELECT id, repository_id, email, confirm_token, unsubscribe_token, confirmed, created_at
		FROM subscriptions WHERE repository_id = $1 AND confirmed = true
	`, repoID)
	if err != nil {
		return nil, fmt.Errorf("get confirmed by repo id: %w", err)
	}

	collectedRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[subscriptionRow])
	if err != nil {
		return nil, fmt.Errorf("collect subscriptions: %w", err)
	}

	return toSubscriptionEntities(collectedRows), nil
}

func (r *SubscriptionRepository) Confirm(ctx context.Context, token string) (*entity.Subscription, string, error) {
	ctx = metrics.WithDBOp(ctx, "confirm", "subscriptions")

	rows, err := r.pool.Query(ctx, `
		WITH target AS (
			SELECT s.id, s.repository_id, s.email, s.unsubscribe_token, s.confirmed AS was_confirmed, r.name AS repo
			FROM subscriptions s
			JOIN repositories r ON r.id = s.repository_id
			WHERE s.confirm_token = $1
		), upd AS (
			UPDATE subscriptions SET confirmed = true WHERE confirm_token = $1
		)
		SELECT id, repository_id, email, unsubscribe_token, was_confirmed, repo FROM target
	`, token)
	if err != nil {
		return nil, "", fmt.Errorf("confirm subscription: %w", err)
	}

	collectedRow, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[confirmRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", entity.ErrNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("confirm subscription: %w", err)
	}

	if collectedRow.WasConfirmed {
		return nil, "", nil
	}

	return collectedRow.toEntity(), collectedRow.Repo, nil
}

func (r *SubscriptionRepository) Delete(ctx context.Context, token string) error {
	ctx = metrics.WithDBOp(ctx, "delete", "subscriptions")

	commandTag, err := r.pool.Exec(ctx, `
		DELETE FROM subscriptions WHERE unsubscribe_token = $1
	`, token)

	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return entity.ErrNotFound
	}

	return nil
}
