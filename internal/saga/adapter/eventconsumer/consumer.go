package eventconsumer

import (
	"context"
	"log/slog"

	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/shared/events"
)

type coordinator interface {
	OnPending(ctx context.Context, sagaID, email, repoName string) error
	OnConfirmationSent(ctx context.Context, sagaID string) error
	OnConfirmationDead(ctx context.Context, sagaID string) error
	OnConfirmed(ctx context.Context, email, repoName string) error
	OnExpired(ctx context.Context, email, repoName string) error
}

type Consumer struct {
	coordinator coordinator
	log         *slog.Logger
}

func New(c coordinator, log *slog.Logger) *Consumer {
	return &Consumer{coordinator: c, log: log.With("component", "saga-event-consumer")}
}

func (c *Consumer) HandlePending(ctx context.Context, data []byte) error {
	ev, err := events.Unmarshal[events.SubscriptionPending](data)
	if err != nil {
		return broker.Terminal(err)
	}

	return c.coordinator.OnPending(logging.WithSagaID(ctx, ev.SagaID), ev.SagaID, ev.Email, ev.RepoName)
}

func (c *Consumer) HandleConfirmationSent(ctx context.Context, data []byte) error {
	ev, err := events.Unmarshal[events.NotificationConfirmationSent](data)
	if err != nil {
		return broker.Terminal(err)
	}

	return c.coordinator.OnConfirmationSent(logging.WithSagaID(ctx, ev.SagaID), ev.SagaID)
}

func (c *Consumer) HandleConfirmationDead(ctx context.Context, data []byte) error {
	ev, err := events.Unmarshal[events.NotificationConfirmationDead](data)
	if err != nil {
		return broker.Terminal(err)
	}

	return c.coordinator.OnConfirmationDead(logging.WithSagaID(ctx, ev.SagaID), ev.SagaID)
}

func (c *Consumer) HandleConfirmed(ctx context.Context, data []byte) error {
	ev, err := events.Unmarshal[events.SubscriptionConfirmed](data)
	if err != nil {
		return broker.Terminal(err)
	}

	return c.coordinator.OnConfirmed(logging.WithSagaID(ctx, ev.SagaID), ev.Email, ev.RepoName)
}

func (c *Consumer) HandleExpired(ctx context.Context, data []byte) error {
	ev, err := events.Unmarshal[events.SubscriptionExpired](data)
	if err != nil {
		return broker.Terminal(err)
	}

	return c.coordinator.OnExpired(logging.WithSagaID(ctx, ev.SagaID), ev.Email, ev.RepoName)
}
