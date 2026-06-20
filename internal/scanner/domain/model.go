package domain

type WatchedRepo struct {
	RepoName        string
	LastSeenTag     *string
	SubscriberCount int
}
