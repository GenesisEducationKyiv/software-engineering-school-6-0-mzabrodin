package notifier

import "github-release-notifier/internal/shared/entity"

// ReleaseNotification is the message the notifier delivers for a new release.
type ReleaseNotification struct {
	To             string
	Repo           string
	Tag            string
	ReleaseURL     string
	UnsubscribeURL string
}

// BatchResult reports the outcome of sending a batch of notifications.
type BatchResult struct {
	Sent   int
	Failed []string
}

// NewReleaseNotification builds a ReleaseNotification for a single subscriber.
func NewReleaseNotification(
	sub *entity.Subscription,
	repo *entity.Repository,
	release *entity.Release,
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
