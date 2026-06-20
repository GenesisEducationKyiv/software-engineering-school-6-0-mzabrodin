package eventpublisher

import (
	"context"
	"fmt"
	"log/slog"

	"github-release-notifier/internal/shared/events"
)

type broker interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

type Publisher struct {
	broker broker
	log    *slog.Logger
}

func New(b broker, log *slog.Logger) *Publisher {
	return &Publisher{broker: b, log: log.With("component", "scanner-publisher")}
}

func (p *Publisher) ReleaseDetected(ctx context.Context, ev events.ReleaseDetected) error {
	return publish(ctx, p.broker, events.SubjectReleaseDetected, ev)
}

func publish[T any](ctx context.Context, b broker, subject string, ev T) error {
	data, err := events.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", subject, err)
	}

	if err := b.Publish(ctx, subject, data); err != nil {
		return fmt.Errorf("publish %s: %w", subject, err)
	}

	return nil
}
