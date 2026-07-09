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
	return &Publisher{relay: r, log: log.With("component", "notifier-publisher")}
}

func (p *Publisher) ConfirmationSent(ctx context.Context, ev events.NotificationConfirmationSent) error {
	return enqueue(ctx, events.SubjectNotificationConfirmationSent, ev)
}

func (p *Publisher) ConfirmationFailed(ctx context.Context, ev events.NotificationConfirmationFailed) error {
	return enqueue(ctx, events.SubjectNotificationConfirmationFailed, ev)
}

func (p *Publisher) ConfirmationDead(ctx context.Context, ev events.NotificationConfirmationDead) error {
	return enqueue(ctx, events.SubjectNotificationConfirmationDead, ev)
}

func (p *Publisher) ReleaseSent(ctx context.Context, ev events.NotificationReleaseSent) error {
	return enqueue(ctx, events.SubjectNotificationReleaseSent, ev)
}

func (p *Publisher) ReleaseFailed(ctx context.Context, ev events.NotificationReleaseFailed) error {
	return enqueue(ctx, events.SubjectNotificationReleaseFailed, ev)
}

func (p *Publisher) ReleaseDead(ctx context.Context, ev events.NotificationReleaseDead) error {
	return enqueue(ctx, events.SubjectNotificationReleaseDead, ev)
}

func (p *Publisher) ReleaseNotified(ctx context.Context, ev events.ReleaseNotified) error {
	return enqueue(ctx, events.SubjectReleaseNotified, ev)
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
