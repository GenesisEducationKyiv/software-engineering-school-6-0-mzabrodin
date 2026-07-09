package repository

import "github-release-notifier/internal/scanner/domain"

type watchedRepoRow struct {
	RepoName        string  `db:"repo_name"`
	LastSeenTag     *string `db:"last_seen_tag"`
	SubscriberCount int     `db:"subscriber_count"`
}

func (row watchedRepoRow) toDomain() domain.WatchedRepo {
	return domain.WatchedRepo{
		RepoName:        row.RepoName,
		LastSeenTag:     row.LastSeenTag,
		SubscriberCount: row.SubscriberCount,
	}
}

func toWatchedRepos(rows []watchedRepoRow) []domain.WatchedRepo {
	repos := make([]domain.WatchedRepo, 0, len(rows))
	for _, row := range rows {
		repos = append(repos, row.toDomain())
	}

	return repos
}
