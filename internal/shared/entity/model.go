package entity

import (
	"time"

	"github.com/google/uuid"
)

type Repository struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
}

type Subscription struct {
	ID               uuid.UUID
	RepositoryID     uuid.UUID
	Email            string
	UnsubscribeToken string
	Confirmed        bool
	CreatedAt        time.Time
}

type Release struct {
	TagName string
	HTMLURL string
}

type ObservedRelease struct {
	Repo    string
	Release *Release
}
