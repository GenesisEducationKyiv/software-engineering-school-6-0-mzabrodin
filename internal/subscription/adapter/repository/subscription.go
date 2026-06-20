package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/metrics"
	shareddomain "github-release-notifier/internal/shared/domain"
	"github-release-notifier/internal/subscription/domain"
)

type SubscriptionRepository struct {
	pool *pgxpool.Pool
}

func NewSubscriptionRepository(pool *pgxpool.Pool) *SubscriptionRepository {
	return &SubscriptionRepository{pool: pool}
}

func (r *SubscriptionRepository) Create(ctx context.Context, sub domain.Subscription) error {
	ctx = metrics.WithDBOp(ctx, "create", "subscriptions")

	commandTag, err := db.FromContext(ctx, r.pool).Exec(ctx, `
		INSERT INTO subscriptions (repository_id, email, unsubscribe_token, confirmed)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email, repository_id) DO NOTHING
	`, sub.RepositoryID, sub.Email, sub.UnsubscribeToken, sub.Confirmed)
	if err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return shareddomain.ErrAlreadyExists
	}

	return nil
}

func (r *SubscriptionRepository) FindByEmailAndRepo(
	ctx context.Context,
	email string,
	repoID uuid.UUID,
) (domain.Subscription, error) {
	ctx = metrics.WithDBOp(ctx, "find_by_email_and_repo", "subscriptions")

	rows, err := db.FromContext(ctx, r.pool).Query(ctx, `
		SELECT id, repository_id, email, unsubscribe_token, confirmed, created_at
		FROM subscriptions WHERE email = $1 AND repository_id = $2
	`, email, repoID)
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("find subscription: %w", err)
	}

	collectedRow, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[subscriptionRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Subscription{}, shareddomain.ErrNotFound
	}
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("find subscription: %w", err)
	}

	return collectedRow.toEntity(), nil
}

func (r *SubscriptionRepository) Confirm(ctx context.Context, email, repo string) (domain.ConfirmResult, error) {
	ctx = metrics.WithDBOp(ctx, "confirm", "subscriptions")

	var unsubToken string

	err := db.FromContext(ctx, r.pool).QueryRow(ctx, `
		UPDATE subscriptions s
		SET confirmed = true
		FROM repositories r
		WHERE s.repository_id = r.id AND r.name = $2 AND s.email = $1 AND s.confirmed = false
		RETURNING s.unsubscribe_token
	`, email, repo).Scan(&unsubToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ConfirmResult{Confirmed: false}, nil
	}
	if err != nil {
		return domain.ConfirmResult{}, fmt.Errorf("confirm subscription: %w", err)
	}

	return domain.ConfirmResult{Confirmed: true, UnsubToken: unsubToken}, nil
}

func (r *SubscriptionRepository) Delete(ctx context.Context, token string) (domain.RemovedSubscription, error) {
	ctx = metrics.WithDBOp(ctx, "delete", "subscriptions")

	var removed domain.RemovedSubscription

	err := db.FromContext(ctx, r.pool).QueryRow(ctx, `
		WITH deleted AS (
			DELETE FROM subscriptions WHERE unsubscribe_token = $1
			RETURNING email, repository_id
		)
		SELECT d.email, r.name FROM deleted d JOIN repositories r ON r.id = d.repository_id
	`, token).Scan(&removed.Email, &removed.Repo)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RemovedSubscription{}, shareddomain.ErrNotFound
	}
	if err != nil {
		return domain.RemovedSubscription{}, fmt.Errorf("delete subscription: %w", err)
	}

	return removed, nil
}

func (r *SubscriptionRepository) DeleteExpiredPending(
	ctx context.Context,
	cutoff time.Time,
) ([]domain.ExpiredSubscription, error) {
	ctx = metrics.WithDBOp(ctx, "delete_expired_pending", "subscriptions")

	rows, err := db.FromContext(ctx, r.pool).Query(ctx, `
		WITH deleted AS (
			DELETE FROM subscriptions
			WHERE confirmed = false AND created_at < $1
			RETURNING email, repository_id
		)
		SELECT d.email, r.name FROM deleted d JOIN repositories r ON r.id = d.repository_id
	`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("delete expired pending: %w", err)
	}

	collectedRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[removedRow])
	if err != nil {
		return nil, fmt.Errorf("collect expired pending: %w", err)
	}

	return toExpiredSubscriptions(collectedRows), nil
}

func (r *SubscriptionRepository) GetByEmail(ctx context.Context, email string) ([]domain.SubscriptionView, error) {
	ctx = metrics.WithDBOp(ctx, "get_by_email", "subscriptions")

	rows, err := db.FromContext(ctx, r.pool).Query(ctx, `
		SELECT s.email, r.name AS repo, s.confirmed
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
