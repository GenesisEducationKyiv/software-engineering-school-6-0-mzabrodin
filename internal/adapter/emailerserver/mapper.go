package emailerserver

import (
	emailerv1 "github-release-notifier/internal/adapter/grpc/gen/emailer/v1"
	"github-release-notifier/internal/entity"
)

func toEntityNotifications(in []*emailerv1.ReleaseNotification) []entity.ReleaseNotification {
	out := make([]entity.ReleaseNotification, len(in))
	for i, n := range in {
		out[i] = entity.ReleaseNotification{
			To:             n.GetTo(),
			Repo:           n.GetRepo(),
			Tag:            n.GetTag(),
			ReleaseURL:     n.GetReleaseUrl(),
			UnsubscribeURL: n.GetUnsubscribeUrl(),
		}
	}

	return out
}
