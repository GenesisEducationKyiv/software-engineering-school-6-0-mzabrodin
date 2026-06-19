package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/notifier/domain"
	"github-release-notifier/internal/shared/entity"
)

type SubscriptionsReadRepository struct {
	pool *pgxpool.Pool
}

func NewSubscriptionsReadRepository(pool *pgxpool.Pool) *SubscriptionsReadRepository {
	return &SubscriptionsReadRepository{pool: pool}
}

func (r *SubscriptionsReadRepository) Upsert(ctx context.Context, email, repoName, unsubToken string) error {
	ctx = metrics.WithDBOp(ctx, "upsert", "subscriptions_read")

	if _, err := r.pool.Exec(ctx, `
		INSERT INTO subscriptions_read (email, repo_name, unsub_token)
		VALUES ($1, $2, $3)
		ON CONFLICT (email, repo_name) DO UPDATE SET unsub_token = EXCLUDED.unsub_token
	`, email, repoName, unsubToken); err != nil {
		return fmt.Errorf("upsert subscriptions_read: %w", err)
	}

	return nil
}

func (r *SubscriptionsReadRepository) Delete(ctx context.Context, email, repoName string) error {
	ctx = metrics.WithDBOp(ctx, "delete", "subscriptions_read")

	if _, err := r.pool.Exec(ctx, `
		DELETE FROM subscriptions_read WHERE email = $1 AND repo_name = $2
	`, email, repoName); err != nil {
		return fmt.Errorf("delete subscriptions_read: %w", err)
	}

	return nil
}

func (r *SubscriptionsReadRepository) RecipientsByRepo(
	ctx context.Context,
	repoName string,
) ([]domain.Recipient, error) {
	ctx = metrics.WithDBOp(ctx, "recipients_by_repo", "subscriptions_read")

	rows, err := r.pool.Query(ctx, `
		SELECT email, unsub_token FROM subscriptions_read WHERE repo_name = $1
	`, repoName)
	if err != nil {
		return nil, fmt.Errorf("query recipients: %w", err)
	}

	collectedRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[recipientRow])
	if err != nil {
		return nil, fmt.Errorf("collect recipients: %w", err)
	}

	return toRecipients(collectedRows), nil
}

func (r *SubscriptionsReadRepository) Recipient(ctx context.Context, email, repoName string) (domain.Recipient, error) {
	ctx = metrics.WithDBOp(ctx, "recipient", "subscriptions_read")

	rows, err := r.pool.Query(ctx, `
		SELECT email, unsub_token FROM subscriptions_read WHERE email = $1 AND repo_name = $2
	`, email, repoName)
	if err != nil {
		return domain.Recipient{}, fmt.Errorf("query recipient: %w", err)
	}

	collectedRow, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[recipientRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Recipient{}, entity.ErrNotFound
	}

	if err != nil {
		return domain.Recipient{}, fmt.Errorf("collect recipient: %w", err)
	}

	return collectedRow.toDomain(), nil
}
