package sendconfirmation

import (
	"context"
	"fmt"
	"log/slog"

	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/domain"
	"github-release-notifier/internal/shared/events"
)

type sender interface {
	DeliverConfirmation(ctx context.Context, to, repo, confirmURL string) error
}

type failedStore interface {
	Add(ctx context.Context, fc *domain.FailedConfirmation) error
}

type publisher interface {
	ConfirmationSent(ctx context.Context, ev events.NotificationConfirmationSent) error
	ConfirmationFailed(ctx context.Context, ev events.NotificationConfirmationFailed) error
}

type Input struct {
	SagaID     string
	Email      string
	RepoName   string
	ConfirmURL string
}

type UseCase struct {
	sender    sender
	failed    failedStore
	publisher publisher
	log       *slog.Logger
}

func New(mailSender sender, failed failedStore, pub publisher, log *slog.Logger) *UseCase {
	return &UseCase{
		sender:    mailSender,
		failed:    failed,
		publisher: pub,
		log:       log.With("component", "send-confirmation"),
	}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) error {
	if err := uc.sender.DeliverConfirmation(ctx, in.Email, in.RepoName, in.ConfirmURL); err != nil {
		uc.log.WarnContext(ctx, "confirmation send failed; queued for retry",
			"email", in.Email, "repo", in.RepoName, "error", err)

		if addErr := uc.failed.Add(ctx, &domain.FailedConfirmation{
			SagaID:     in.SagaID,
			Email:      in.Email,
			RepoName:   in.RepoName,
			ConfirmURL: in.ConfirmURL,
			Reason:     err.Error(),
		}); addErr != nil {
			return fmt.Errorf("record confirmation failure: %w", addErr)
		}

		notifier.TryPublish(ctx, uc.log, "confirmation failed", func() error {
			return uc.publisher.ConfirmationFailed(ctx, events.NotificationConfirmationFailed{
				SagaID: in.SagaID,
				Email:  in.Email,
				Reason: err.Error(),
			})
		})

		return nil
	}

	notifier.TryPublish(ctx, uc.log, "confirmation sent", func() error {
		return uc.publisher.ConfirmationSent(ctx, events.NotificationConfirmationSent{
			SagaID: in.SagaID,
			Email:  in.Email,
		})
	})

	return nil
}
