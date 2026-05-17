package domain

import (
	"time"

	"github.com/google/uuid"
)

type Repository struct {
	ID          uuid.UUID
	Name        string
	LastSeenTag *string
	CheckedAt   *time.Time
	CreatedAt   time.Time
}

type Subscription struct {
	ID               uuid.UUID
	RepositoryID     uuid.UUID
	Email            string
	ConfirmToken     string
	UnsubscribeToken string
	Confirmed        bool
	CreatedAt        time.Time
}

type SubscriptionView struct {
	Email       string
	Repo        string
	Confirmed   bool
	LastSeenTag *string
}

type ReleaseNotification struct {
	To             string
	Repo           string
	Tag            string
	ReleaseURL     string
	UnsubscribeURL string
}

func NewReleaseNotification(
	sub *Subscription,
	repo *Repository,
	release *Release,
	unsubURL string,
) ReleaseNotification {
	return ReleaseNotification{
		To:             sub.Email,
		Repo:           repo.Name,
		Tag:            release.TagName,
		ReleaseURL:     release.HTMLURL,
		UnsubscribeURL: unsubURL,
	}
}

type Release struct {
	TagName string
	HTMLURL string
}
