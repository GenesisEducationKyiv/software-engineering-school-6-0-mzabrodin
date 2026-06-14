package notifierclient

import (
	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/grpc/gen/notifierv1"
)

func toProtoNotifications(notifications []notifier.ReleaseNotification) []*notifierv1.ReleaseNotification {
	out := make([]*notifierv1.ReleaseNotification, len(notifications))
	for i := range notifications {
		n := &notifications[i]
		out[i] = &notifierv1.ReleaseNotification{
			To:             n.To,
			Repo:           n.Repo,
			Tag:            n.Tag,
			ReleaseUrl:     n.ReleaseURL,
			UnsubscribeUrl: n.UnsubscribeURL,
		}
	}

	return out
}

func recipients(notifications []notifier.ReleaseNotification) []string {
	out := make([]string, len(notifications))
	for i := range notifications {
		out[i] = notifications[i].To
	}

	return out
}
