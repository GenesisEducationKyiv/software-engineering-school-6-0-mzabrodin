package eventconsumer

import (
	"context"
	"log/slog"

	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/subscription/usecase/compensate"
)

type compensator interface {
	Execute(ctx context.Context, in compensate.Input) (bool, error)
}

type Consumer struct {
	compensate compensator
	log        *slog.Logger
}

func New(c compensator, log *slog.Logger) *Consumer {
	return &Consumer{compensate: c, log: log.With("component", "subscription-event-consumer")}
}

func (c *Consumer) HandleCompensate(ctx context.Context, data []byte) error {
	ev, err := events.Unmarshal[events.SagaCompensate](data)
	if err != nil {
		return broker.Terminal(err)
	}

	_, err = c.compensate.Execute(logging.WithSagaID(ctx, ev.SagaID), compensate.Input{
		Email:    ev.Email,
		RepoName: ev.RepoName,
	})

	return err
}
