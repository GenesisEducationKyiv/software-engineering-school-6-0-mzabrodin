package scanner

import (
	"context"
	"log/slog"

	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/scanner/adapter/eventconsumer"
	"github-release-notifier/internal/shared/events"
)

func startConsumers(
	ctx context.Context,
	conn *broker.Conn,
	ec *eventconsumer.Consumer,
	log *slog.Logger,
) (func(), error) {
	return bootstrap.StartConsumers(ctx, conn, []bootstrap.ConsumerSpec{
		{
			Stream:  events.StreamSubscriptions,
			Durable: "scanner-subscription-confirmed",
			Subject: events.SubjectSubscriptionConfirmed,
			Handler: ec.HandleConfirmed,
		},
		{
			Stream:  events.StreamSubscriptions,
			Durable: "scanner-subscription-removed",
			Subject: events.SubjectSubscriptionRemoved,
			Handler: ec.HandleRemoved,
		},
		{
			Stream:  events.StreamReleases,
			Durable: "scanner-release-notified",
			Subject: events.SubjectReleaseNotified,
			Handler: ec.HandleReleaseNotified,
		},
	}, log)
}
