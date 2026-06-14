package notifierconsumer

import (
	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/grpc/gen/notifierv1"
)

func toEntityNotifications(in []*notifierv1.ReleaseNotification) []notifier.ReleaseNotification {
	out := make([]notifier.ReleaseNotification, len(in))
	for i, n := range in {
		out[i] = notifier.ReleaseNotification{
			To:             n.GetTo(),
			Repo:           n.GetRepo(),
			Tag:            n.GetTag(),
			ReleaseURL:     n.GetReleaseUrl(),
			UnsubscribeURL: n.GetUnsubscribeUrl(),
		}
	}

	return out
}
