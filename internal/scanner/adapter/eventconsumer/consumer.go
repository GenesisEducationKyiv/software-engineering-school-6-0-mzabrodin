package eventconsumer

import (
	"context"
	"log/slog"

	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/scanner/usecase/advancetag"
	"github-release-notifier/internal/shared/events"
)

type watchlistProjector interface {
	Confirmed(ctx context.Context, ev events.SubscriptionConfirmed) error
	Removed(ctx context.Context, ev events.SubscriptionRemoved) error
}

type tagAdvancer interface {
	Execute(ctx context.Context, in advancetag.Input) error
}

type Consumer struct {
	projector watchlistProjector
	advancer  tagAdvancer
	log       *slog.Logger
}

func New(projector watchlistProjector, advancer tagAdvancer, log *slog.Logger) *Consumer {
	return &Consumer{
		projector: projector,
		advancer:  advancer,
		log:       log.With("component", "scanner-event-consumer"),
	}
}

func (c *Consumer) HandleConfirmed(ctx context.Context, data []byte) error {
	ev, err := events.Unmarshal[events.SubscriptionConfirmed](data)
	if err != nil {
		return broker.Terminal(err)
	}

	return c.projector.Confirmed(ctx, ev)
}

func (c *Consumer) HandleRemoved(ctx context.Context, data []byte) error {
	ev, err := events.Unmarshal[events.SubscriptionRemoved](data)
	if err != nil {
		return broker.Terminal(err)
	}

	return c.projector.Removed(ctx, ev)
}

func (c *Consumer) HandleReleaseNotified(ctx context.Context, data []byte) error {
	ev, err := events.Unmarshal[events.ReleaseNotified](data)
	if err != nil {
		return broker.Terminal(err)
	}

	return c.advancer.Execute(ctx, advancetag.Input{
		RepoName:  ev.RepoName,
		Tag:       ev.Tag,
		SentCount: ev.SentCount,
	})
}
