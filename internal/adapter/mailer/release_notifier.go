package mailer

import (
	"context"

	"github-release-notifier/internal/entity"
)

type releaseSender interface {
	SendReleaseNotifications(ctx context.Context, notifications []entity.ReleaseNotification) error
}

type unsubscribeURLBuilder interface {
	UnsubscribeURL(token string) string
}

type ReleaseNotifier struct {
	sender releaseSender
	urls   unsubscribeURLBuilder
}

func NewReleaseNotifier(sender releaseSender, urls unsubscribeURLBuilder) *ReleaseNotifier {
	return &ReleaseNotifier{sender: sender, urls: urls}
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

	return n.sender.SendReleaseNotifications(ctx, notifications)
}
