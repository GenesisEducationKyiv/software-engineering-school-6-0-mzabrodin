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
	return &Publisher{relay: r, log: log.With("component", "subscription-publisher")}
}

func (p *Publisher) SubscriptionPending(ctx context.Context, ev events.SubscriptionPending) error {
	return enqueue(ctx, events.SubjectSubscriptionPending, ev)
}

func (p *Publisher) SubscriptionConfirmed(ctx context.Context, ev events.SubscriptionConfirmed) error {
	return enqueue(ctx, events.SubjectSubscriptionConfirmed, ev)
}

func (p *Publisher) SubscriptionRemoved(ctx context.Context, ev events.SubscriptionRemoved) error {
	return enqueue(ctx, events.SubjectSubscriptionRemoved, ev)
}

func (p *Publisher) SubscriptionExpired(ctx context.Context, ev events.SubscriptionExpired) error {
	return enqueue(ctx, events.SubjectSubscriptionExpired, ev)
}

func (p *Publisher) Notify() {
	p.relay.Notify()
}

func enqueue[T any](ctx context.Context, subject string, ev T) error {
	tx, ok := db.Tx(ctx)
	if !ok {
		return errNoTx
	}

	data, err := events.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", subject, err)
	}

	return outbox.Enqueue(ctx, tx, subject, data)
}
