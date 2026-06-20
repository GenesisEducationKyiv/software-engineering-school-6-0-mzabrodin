package notifier

import (
	"context"
	"fmt"
	"log/slog"

	"buf.build/go/protovalidate"

	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/adapter/eventconsumer"
	"github-release-notifier/internal/notifier/adapter/mailer"
	"github-release-notifier/internal/notifier/adapter/notifierconsumer"
	"github-release-notifier/internal/shared/events"
)

func startConsumers(
	ctx context.Context,
	conn *broker.Conn,
	mail *mailer.Mailer,
	ec *eventconsumer.Consumer,
	log *slog.Logger,
) (func(), error) {
	validator, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("create validator: %w", err)
	}

	legacy := notifierconsumer.New(mail, validator, log)

	return bootstrap.StartConsumers(ctx, conn, []bootstrap.ConsumerSpec{
		{
			Stream:  notifier.StreamEmail,
			Durable: "notifier-confirmation",
			Subject: notifier.SubjectConfirmation,
			Handler: legacy.HandleConfirmation,
		},
		{
			Stream:  notifier.StreamEmail,
			Durable: "notifier-release",
			Subject: notifier.SubjectRelease,
			Handler: legacy.HandleRelease,
		},
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
