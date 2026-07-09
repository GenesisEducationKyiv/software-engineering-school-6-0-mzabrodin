package saga

import (
	"context"
	"log/slog"

	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/saga/adapter/eventconsumer"
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
			Durable: "saga-subscription-pending",
			Subject: events.SubjectSubscriptionPending,
			Handler: ec.HandlePending,
		},
		{
			Stream:  events.StreamSubscriptions,
			Durable: "saga-subscription-confirmed",
			Subject: events.SubjectSubscriptionConfirmed,
			Handler: ec.HandleConfirmed,
		},
		{
			Stream:  events.StreamSubscriptions,
			Durable: "saga-subscription-expired",
			Subject: events.SubjectSubscriptionExpired,
			Handler: ec.HandleExpired,
		},
		{
			Stream:  events.StreamNotifications,
			Durable: "saga-notification-confirmation-sent",
			Subject: events.SubjectNotificationConfirmationSent,
			Handler: ec.HandleConfirmationSent,
		},
		{
			Stream:  events.StreamNotifications,
			Durable: "saga-notification-confirmation-dead",
			Subject: events.SubjectNotificationConfirmationDead,
			Handler: ec.HandleConfirmationDead,
		},
	}, log)
}
