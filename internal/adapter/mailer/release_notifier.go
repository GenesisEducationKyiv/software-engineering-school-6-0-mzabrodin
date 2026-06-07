package mailer

import (
	"context"
	"fmt"
	"log/slog"

	"github-release-notifier/internal/entity"
)

type releaseSender interface {
	SendReleaseNotifications(ctx context.Context, notifications []entity.ReleaseNotification) entity.BatchResult
}

type unsubscribeURLBuilder interface {
	UnsubscribeURL(token string) string
}

type ReleaseNotifier struct {
	sender releaseSender
	urls   unsubscribeURLBuilder
	log    *slog.Logger
}

func NewReleaseNotifier(sender releaseSender, urls unsubscribeURLBuilder, log *slog.Logger) *ReleaseNotifier {
	return &ReleaseNotifier{sender: sender, urls: urls, log: log.With("component", "release_notifier")}
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

	result := n.sender.SendReleaseNotifications(ctx, notifications)

	if len(result.Failed) > 0 {
		n.log.WarnContext(ctx, "some release notifications failed",
			"repo", repo.Name, "tag", release.TagName, "sent", result.Sent, "failed", len(result.Failed))
	}

	if result.Sent == 0 && len(result.Failed) > 0 {
		return fmt.Errorf("all %d release notifications failed", len(result.Failed))
	}

	return nil
}
