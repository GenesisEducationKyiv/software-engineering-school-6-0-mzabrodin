package scanner

import (
	"context"

	"github-release-notifier/internal/domain"
)

type mailer interface {
	SendReleaseNotifications(ctx context.Context, notifications []domain.ReleaseNotification) error
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
	subs []*domain.Subscription,
	repo *domain.Repository,
	release *domain.Release,
) error {
	notifications := make([]domain.ReleaseNotification, 0, len(subs))
	for _, sub := range subs {
		notifications = append(notifications,
			domain.NewReleaseNotification(sub, repo, release, n.urls.UnsubscribeURL(sub.UnsubscribeToken)),
		)
	}

	return n.mailer.SendReleaseNotifications(ctx, notifications)
}
