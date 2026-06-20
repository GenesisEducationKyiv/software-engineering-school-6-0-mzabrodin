package eventconsumer

import (
	"context"
	"log/slog"

	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/notifier/usecase/notifyrelease"
	"github-release-notifier/internal/notifier/usecase/sendconfirmation"
	"github-release-notifier/internal/shared/events"
)

type confirmationSender interface {
	Execute(ctx context.Context, in sendconfirmation.Input) error
}

type readModelProjector interface {
	Confirmed(ctx context.Context, ev events.SubscriptionConfirmed) error
	Removed(ctx context.Context, ev events.SubscriptionRemoved) error
}

type releaseNotifier interface {
	Execute(ctx context.Context, in notifyrelease.Input) (notifyrelease.Output, error)
}

type Consumer struct {
	confirmation confirmationSender
	projector    readModelProjector
	release      releaseNotifier
	log          *slog.Logger
}

func New(
	confirmation confirmationSender,
	projector readModelProjector,
	release releaseNotifier,
	log *slog.Logger,
) *Consumer {
	return &Consumer{
		confirmation: confirmation,
		projector:    projector,
		release:      release,
		log:          log.With("component", "notifier-event-consumer"),
	}
}

func (c *Consumer) HandlePending(ctx context.Context, data []byte) error {
	ev, err := events.Unmarshal[events.SubscriptionPending](data)
	if err != nil {
		return broker.Terminal(err)
	}

	return c.confirmation.Execute(ctx, sendconfirmation.Input{
		SagaID:     ev.SagaID,
		Email:      ev.Email,
		RepoName:   ev.RepoName,
		ConfirmURL: ev.ConfirmURL,
	})
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

func (c *Consumer) HandleReleaseDetected(ctx context.Context, data []byte) error {
	ev, err := events.Unmarshal[events.ReleaseDetected](data)
	if err != nil {
		return broker.Terminal(err)
	}

	_, err = c.release.Execute(ctx, notifyrelease.Input{
		SagaID:     ev.SagaID,
		RepoName:   ev.RepoName,
		Tag:        ev.Tag,
		ReleaseURL: ev.ReleaseURL,
	})

	return err
}
