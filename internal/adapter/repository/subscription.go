package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/entity"
)

type SubscriptionRepository struct {
	pool *pgxpool.Pool
}

func NewSubscriptionRepository(pool *pgxpool.Pool) *SubscriptionRepository {
	return &SubscriptionRepository{pool: pool}
}

func (r *SubscriptionRepository) Create(ctx context.Context, sub *entity.Subscription) (err error) {
	start := time.Now()
	defer func() { trackDBQuery(start, "create", "subscriptions", err) }()

	err = r.pool.QueryRow(ctx, `
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
) (views []*entity.SubscriptionView, err error) {
	start := time.Now()
	defer func() { trackDBQuery(start, "get_by_email", "subscriptions", err) }()

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

	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[subscriptionViewRow])
	if err != nil {
		return nil, fmt.Errorf("collect subscription views: %w", err)
	}

	return toSubscriptionViewEntities(collected), nil
}

func (r *SubscriptionRepository) GetConfirmedByRepoID(
	ctx context.Context,
	repoID uuid.UUID,
) (subs []*entity.Subscription, err error) {
	start := time.Now()
	defer func() { trackDBQuery(start, "get_confirmed_by_repo_id", "subscriptions", err) }()

	rows, err := r.pool.Query(ctx, `
		SELECT id, repository_id, email, confirm_token, unsubscribe_token, confirmed, created_at
		FROM subscriptions WHERE repository_id = $1 AND confirmed = true
	`, repoID)

	if err != nil {
		return nil, fmt.Errorf("get confirmed by repo id: %w", err)
	}

	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[subscriptionRow])
	if err != nil {
		return nil, fmt.Errorf("collect subscriptions: %w", err)
	}

	return toSubscriptionEntities(collected), nil
}

func (r *SubscriptionRepository) Confirm(ctx context.Context, token string) (err error) {
	start := time.Now()
	defer func() { trackDBQuery(start, "confirm", "subscriptions", err) }()

	result, err := r.pool.Exec(ctx, `
		UPDATE subscriptions SET confirmed = true WHERE confirm_token = $1
	`, token)

	if err != nil {
		return fmt.Errorf("confirm subscription: %w", err)
	}

	if result.RowsAffected() == 0 {
		return entity.ErrNotFound
	}

	return nil
}

func (r *SubscriptionRepository) Delete(ctx context.Context, token string) (err error) {
	start := time.Now()
	defer func() { trackDBQuery(start, "delete", "subscriptions", err) }()

	result, err := r.pool.Exec(ctx, `
		DELETE FROM subscriptions WHERE unsubscribe_token = $1
	`, token)

	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}

	if result.RowsAffected() == 0 {
		return entity.ErrNotFound
	}

	return nil
}
