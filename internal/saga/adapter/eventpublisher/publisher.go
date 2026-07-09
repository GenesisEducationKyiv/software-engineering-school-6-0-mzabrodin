package eventpublisher

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/outbox"
	"github-release-notifier/internal/shared/events"
)

var errNoTx = errors.New("eventpublisher: enqueue must run within a transaction")

type relay interface {
	Notify()
}

type Publisher struct {
	relay relay
	log   *slog.Logger
}

func New(r relay, log *slog.Logger) *Publisher {
	return &Publisher{relay: r, log: log.With("component", "saga-publisher")}
}

func (p *Publisher) Compensate(ctx context.Context, ev events.SagaCompensate) error {
	tx, ok := db.Tx(ctx)
	if !ok {
		return errNoTx
	}

	data, err := events.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", events.SubjectSagaCompensate, err)
	}

	return outbox.Enqueue(ctx, tx, events.SubjectSagaCompensate, data)
}

func (p *Publisher) Notify() {
	p.relay.Notify()
}
