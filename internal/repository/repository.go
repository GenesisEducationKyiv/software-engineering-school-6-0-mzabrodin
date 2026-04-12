package repository

import (
	"context"
	"github-release-notifier/internal/domain"

	"github.com/google/uuid"
)

type RepositoriesRepository interface {
	Create(ctx context.Context, repo *domain.Repository) error
	GetByName(ctx context.Context, name string) (*domain.Repository, error)
	GetAllWithSubscriptions(ctx context.Context) ([]*domain.Repository, error)
	UpdateLastSeenTag(ctx context.Context, name string, tag string) error
}

type SubscriptionsRepository interface {
	Create(ctx context.Context, sub *domain.Subscription) error
	GetByConfirmToken(ctx context.Context, token string) (*domain.Subscription, error)
	GetByUnsubscribeToken(ctx context.Context, token string) (*domain.Subscription, error)
	GetByEmail(ctx context.Context, email string) ([]*domain.SubscriptionView, error)
	GetConfirmedByRepoID(ctx context.Context, repoID uuid.UUID) ([]*domain.Subscription, error)
	Confirm(ctx context.Context, token string) error
	Delete(ctx context.Context, token string) error
}
