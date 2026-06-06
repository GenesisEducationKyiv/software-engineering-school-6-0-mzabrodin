package scanner

import (
	"context"

	"github-release-notifier/internal/entity"
)

type mailer interface {
	SendReleaseNotifications(ctx context.Context, notifications []entity.ReleaseNotification) error
}

type urlBuilder interface {
	UnsubscribeURL(token string) string
}

type ReleaseNotifier struct {
	mailer mailer
	urls   urlBuilder
}

func NewReleaseNotifier(m mailer, urls urlBuilder) *ReleaseNotifier {
	return &ReleaseNotifier{mailer: m, urls: urls}
}

func (n *ReleaseNotifier) Notify(
	ctx context.Context,
	subs []*entity.Subscription,
	repo *entity.Repository,
	release *entity.Release,
) error {
	notifications := make([]entity.ReleaseNotification, 0, len(subs))
	for _, sub := range subs {
		notifications = append(notifications,
			entity.NewReleaseNotification(sub, repo, release, n.urls.UnsubscribeURL(sub.UnsubscribeToken)),
		)
	}

	return n.mailer.SendReleaseNotifications(ctx, notifications)
}
