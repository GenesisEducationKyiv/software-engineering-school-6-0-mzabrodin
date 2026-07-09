package subscription

import (
	"context"
	"log/slog"

	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/subscription/adapter/eventconsumer"
)

func startConsumers(
	ctx context.Context,
	conn *broker.Conn,
	ec *eventconsumer.Consumer,
	log *slog.Logger,
) (func(), error) {
	return bootstrap.StartConsumers(ctx, conn, []bootstrap.ConsumerSpec{
		{
			Stream:  events.StreamSagas,
			Durable: "subscription-saga-compensate",
			Subject: events.SubjectSagaCompensate,
			Handler: ec.HandleCompensate,
		},
	}, log)
}
