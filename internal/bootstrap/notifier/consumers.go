package notifier

import (
	"context"
	"log/slog"

	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/notifier/adapter/eventconsumer"
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
			Durable: "notifier-subscription-pending",
			Subject: events.SubjectSubscriptionPending,
			Handler: ec.HandlePending,
		},
		{
			Stream:  events.StreamSubscriptions,
			Durable: "notifier-subscription-confirmed",
			Subject: events.SubjectSubscriptionConfirmed,
			Handler: ec.HandleConfirmed,
		},
		{
			Stream:  events.StreamSubscriptions,
			Durable: "notifier-subscription-removed",
			Subject: events.SubjectSubscriptionRemoved,
			Handler: ec.HandleRemoved,
		},
		{
			Stream:  events.StreamReleases,
			Durable: "notifier-release-detected",
			Subject: events.SubjectReleaseDetected,
			Handler: ec.HandleReleaseDetected,
		},
	}, log)
}
