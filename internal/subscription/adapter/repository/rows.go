package repository

import (
	"time"

	"github.com/google/uuid"

	"github-release-notifier/internal/subscription/domain"
)

type repositoryRow struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

func (row repositoryRow) toEntity() domain.Repository {
	return domain.Repository{
		ID:        row.ID,
		Name:      row.Name,
		CreatedAt: row.CreatedAt,
	}
}

type subscriptionRow struct {
	ID               uuid.UUID `db:"id"`
	RepositoryID     uuid.UUID `db:"repository_id"`
	Email            string    `db:"email"`
	UnsubscribeToken string    `db:"unsubscribe_token"`
	Confirmed        bool      `db:"confirmed"`
	CreatedAt        time.Time `db:"created_at"`
}

func (row subscriptionRow) toEntity() domain.Subscription {
	return domain.Subscription{
		ID:               row.ID,
		RepositoryID:     row.RepositoryID,
		Email:            row.Email,
		UnsubscribeToken: row.UnsubscribeToken,
		Confirmed:        row.Confirmed,
		CreatedAt:        row.CreatedAt,
	}
}

type subscriptionViewRow struct {
	Email     string `db:"email"`
	Repo      string `db:"repo"`
	Confirmed bool   `db:"confirmed"`
}

func (row subscriptionViewRow) toEntity() domain.SubscriptionView {
	return domain.SubscriptionView{
		Email:     row.Email,
		Repo:      row.Repo,
		Confirmed: row.Confirmed,
	}
}

type removedRow struct {
	Email string `db:"email"`
	Repo  string `db:"repo"`
}

func toSubscriptionViewEntities(rows []subscriptionViewRow) []domain.SubscriptionView {
	views := make([]domain.SubscriptionView, 0, len(rows))
	for _, row := range rows {
		views = append(views, row.toEntity())
	}

	return views
}

func toExpiredSubscriptions(rows []removedRow) []domain.ExpiredSubscription {
	expired := make([]domain.ExpiredSubscription, 0, len(rows))
	for _, row := range rows {
		expired = append(expired, domain.ExpiredSubscription{Email: row.Email, Repo: row.Repo})
	}

	return expired
}
