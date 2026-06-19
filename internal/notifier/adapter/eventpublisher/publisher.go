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
	return &Publisher{broker: b, log: log.With("component", "notifier-publisher")}
}

func (p *Publisher) ConfirmationSent(ctx context.Context, ev events.NotificationConfirmationSent) error {
	return publish(ctx, p.broker, events.SubjectNotificationConfirmationSent, ev)
}

func (p *Publisher) ConfirmationFailed(ctx context.Context, ev events.NotificationConfirmationFailed) error {
	return publish(ctx, p.broker, events.SubjectNotificationConfirmationFailed, ev)
}

func (p *Publisher) ConfirmationDead(ctx context.Context, ev events.NotificationConfirmationDead) error {
	return publish(ctx, p.broker, events.SubjectNotificationConfirmationDead, ev)
}

func (p *Publisher) ReleaseSent(ctx context.Context, ev events.NotificationReleaseSent) error {
	return publish(ctx, p.broker, events.SubjectNotificationReleaseSent, ev)
}

func (p *Publisher) ReleaseFailed(ctx context.Context, ev events.NotificationReleaseFailed) error {
	return publish(ctx, p.broker, events.SubjectNotificationReleaseFailed, ev)
}

func (p *Publisher) ReleaseDead(ctx context.Context, ev events.NotificationReleaseDead) error {
	return publish(ctx, p.broker, events.SubjectNotificationReleaseDead, ev)
}

func (p *Publisher) ReleaseNotified(ctx context.Context, ev events.ReleaseNotified) error {
	return publish(ctx, p.broker, events.SubjectReleaseNotified, ev)
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
