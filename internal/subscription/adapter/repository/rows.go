package repository

import (
	"time"

	"github.com/google/uuid"

	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/subscription/domain"
)

type repositoryRow struct {
	ID          uuid.UUID  `db:"id"`
	Name        string     `db:"name"`
	LastSeenTag *string    `db:"last_seen_tag"`
	CheckedAt   *time.Time `db:"checked_at"`
	CreatedAt   time.Time  `db:"created_at"`
}

func (row repositoryRow) toEntity() *entity.Repository {
	return &entity.Repository{
		ID:          row.ID,
		Name:        row.Name,
		LastSeenTag: row.LastSeenTag,
		CheckedAt:   row.CheckedAt,
		CreatedAt:   row.CreatedAt,
	}
}

type subscriptionRow struct {
	ID               uuid.UUID `db:"id"`
	RepositoryID     uuid.UUID `db:"repository_id"`
	Email            string    `db:"email"`
	ConfirmToken     string    `db:"confirm_token"`
	UnsubscribeToken string    `db:"unsubscribe_token"`
	Confirmed        bool      `db:"confirmed"`
	CreatedAt        time.Time `db:"created_at"`
}

func (row *subscriptionRow) toEntity() *entity.Subscription {
	return &entity.Subscription{
		ID:               row.ID,
		RepositoryID:     row.RepositoryID,
		Email:            row.Email,
		ConfirmToken:     row.ConfirmToken,
		UnsubscribeToken: row.UnsubscribeToken,
		Confirmed:        row.Confirmed,
		CreatedAt:        row.CreatedAt,
	}
}

type confirmRow struct {
	ID               uuid.UUID `db:"id"`
	RepositoryID     uuid.UUID `db:"repository_id"`
	Email            string    `db:"email"`
	UnsubscribeToken string    `db:"unsubscribe_token"`
	WasConfirmed     bool      `db:"was_confirmed"`
	Repo             string    `db:"repo"`
}

func (row *confirmRow) toEntity() *entity.Subscription {
	return &entity.Subscription{
		ID:               row.ID,
		RepositoryID:     row.RepositoryID,
		Email:            row.Email,
		UnsubscribeToken: row.UnsubscribeToken,
		Confirmed:        true,
	}
}

type subscriptionViewRow struct {
	Email       string  `db:"email"`
	Repo        string  `db:"repo"`
	Confirmed   bool    `db:"confirmed"`
	LastSeenTag *string `db:"last_seen_tag"`
}

func (row subscriptionViewRow) toEntity() *domain.SubscriptionView {
	return &domain.SubscriptionView{
		Email:       row.Email,
		Repo:        row.Repo,
		Confirmed:   row.Confirmed,
		LastSeenTag: row.LastSeenTag,
	}
}

func toRepositoryEntities(rows []repositoryRow) []*entity.Repository {
	repos := make([]*entity.Repository, 0, len(rows))
	for _, row := range rows {
		repos = append(repos, row.toEntity())
	}

	return repos
}

func toSubscriptionEntities(rows []subscriptionRow) []*entity.Subscription {
	subs := make([]*entity.Subscription, 0, len(rows))
	for _, row := range rows {
		subs = append(subs, row.toEntity())
	}

	return subs
}

func toSubscriptionViewEntities(rows []subscriptionViewRow) []*domain.SubscriptionView {
	views := make([]*domain.SubscriptionView, 0, len(rows))
	for _, row := range rows {
		views = append(views, row.toEntity())
	}

	return views
}
