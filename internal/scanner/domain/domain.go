package domain

import shareddomain "github-release-notifier/internal/shared/domain"

type WatchedRepo struct {
	RepoName        string
	LastSeenTag     *string
	SubscriberCount int
}

type ObservedRelease struct {
	Repo    string
	Release *shareddomain.Release
}
