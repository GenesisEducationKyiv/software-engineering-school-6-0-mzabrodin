package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/domain"
)

type SubscriptionRepository struct {
	pool *pgxpool.Pool
}

func NewSubscriptionRepository(pool *pgxpool.Pool) *SubscriptionRepository {
	return &SubscriptionRepository{pool: pool}
}

func (r *SubscriptionRepository) Create(ctx context.Context, sub *domain.Subscription) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO subscriptions (repository_id, email, confirm_token, unsubscribe_token, confirmed)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email, repository_id) DO NOTHING
		RETURNING id, created_at
	`, sub.RepositoryID, sub.Email, sub.ConfirmToken, sub.UnsubscribeToken, sub.Confirmed).Scan(&sub.ID, &sub.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrAlreadyExists
	}

	if err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}

	return nil
}

func (r *SubscriptionRepository) GetByEmail(ctx context.Context, email string) ([]*domain.SubscriptionView, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.email, r.name, s.confirmed, r.last_seen_tag
		FROM subscriptions s
		JOIN repositories r ON r.id = s.repository_id
		WHERE s.email = $1
		ORDER BY s.created_at DESC
	`, email)

	if err != nil {
		return nil, fmt.Errorf("get by email: %w", err)
	}

	defer rows.Close()

	var views []*domain.SubscriptionView
	for rows.Next() {
		view := &domain.SubscriptionView{}
		if err := rows.Scan(&view.Email, &view.Repo, &view.Confirmed, &view.LastSeenTag); err != nil {
			return nil, fmt.Errorf("scan subscription view: %w", err)
		}
		views = append(views, view)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return views, nil
}

func (r *SubscriptionRepository) GetConfirmedByRepoID(
	ctx context.Context,
	repoID uuid.UUID,
) ([]*domain.Subscription, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, repository_id, email, confirm_token, unsubscribe_token, confirmed, created_at
		FROM subscriptions WHERE repository_id = $1 AND confirmed = true
	`, repoID)

	if err != nil {
		return nil, fmt.Errorf("get confirmed by repo id: %w", err)
	}

	defer rows.Close()

	var subs []*domain.Subscription
	for rows.Next() {
		sub := &domain.Subscription{}
		if err := rows.Scan(
			&sub.ID, &sub.RepositoryID, &sub.Email,
			&sub.ConfirmToken, &sub.UnsubscribeToken,
			&sub.Confirmed, &sub.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}

		subs = append(subs, sub)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return subs, nil
}

func (r *SubscriptionRepository) Confirm(ctx context.Context, token string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE subscriptions SET confirmed = true WHERE confirm_token = $1
	`, token)

	if err != nil {
		return fmt.Errorf("confirm subscription: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *SubscriptionRepository) Delete(ctx context.Context, token string) error {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM subscriptions WHERE unsubscribe_token = $1
	`, token)

	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
