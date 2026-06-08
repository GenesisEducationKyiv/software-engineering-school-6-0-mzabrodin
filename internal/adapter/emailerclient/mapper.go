package emailerclient

import (
	emailerv1 "github-release-notifier/internal/adapter/grpc/gen/emailer/v1"
	"github-release-notifier/internal/entity"
)

func toProtoNotifications(notifications []entity.ReleaseNotification) []*emailerv1.ReleaseNotification {
	out := make([]*emailerv1.ReleaseNotification, len(notifications))
	for i := range notifications {
		n := &notifications[i]
		out[i] = &emailerv1.ReleaseNotification{
			To:             n.To,
			Repo:           n.Repo,
			Tag:            n.Tag,
			ReleaseUrl:     n.ReleaseURL,
			UnsubscribeUrl: n.UnsubscribeURL,
		}
	}

	return out
}

func recipients(notifications []entity.ReleaseNotification) []string {
	out := make([]string, len(notifications))
	for i := range notifications {
		out[i] = notifications[i].To
	}

	return out
}
