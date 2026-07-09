package sendconfirmation

import (
	"context"
	"fmt"
	"log/slog"

	"github-release-notifier/internal/notifier/domain"
	"github-release-notifier/internal/shared/events"
)

type sender interface {
	DeliverConfirmation(ctx context.Context, to, repo, confirmURL string) error
}

type failedStore interface {
	Add(ctx context.Context, fc *domain.FailedConfirmation) error
}

type transactor interface {
	Within(ctx context.Context, fn func(ctx context.Context) error) error
}

type publisher interface {
	ConfirmationSent(ctx context.Context, ev events.NotificationConfirmationSent) error
	ConfirmationFailed(ctx context.Context, ev events.NotificationConfirmationFailed) error
	Notify()
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
	tx        transactor
	publisher publisher
	log       *slog.Logger
}

func New(mailSender sender, failed failedStore, tx transactor, pub publisher, log *slog.Logger) *UseCase {
	return &UseCase{
		sender:    mailSender,
		failed:    failed,
		tx:        tx,
		publisher: pub,
		log:       log.With("component", "send-confirmation"),
	}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) error {
	uc.log.InfoContext(ctx, "sending confirmation email", "email", in.Email, "repo", in.RepoName)

	if err := uc.sender.DeliverConfirmation(ctx, in.Email, in.RepoName, in.ConfirmURL); err != nil {
		uc.log.WarnContext(ctx, "confirmation send failed; queued for retry",
			"email", in.Email, "repo", in.RepoName, "error", err)

		if txErr := uc.tx.Within(ctx, func(txCtx context.Context) error {
			if addErr := uc.failed.Add(txCtx, &domain.FailedConfirmation{
				SagaID:     in.SagaID,
				Email:      in.Email,
				RepoName:   in.RepoName,
				ConfirmURL: in.ConfirmURL,
				Reason:     err.Error(),
			}); addErr != nil {
				return addErr
			}

			return uc.publisher.ConfirmationFailed(txCtx, events.NotificationConfirmationFailed{
				SagaID: in.SagaID,
				Email:  in.Email,
				Reason: err.Error(),
			})
		}); txErr != nil {
			return fmt.Errorf("record confirmation failure: %w", txErr)
		}

		uc.publisher.Notify()

		return nil
	}

	uc.log.InfoContext(ctx, "confirmation email sent", "email", in.Email, "repo", in.RepoName)

	if err := uc.tx.Within(ctx, func(txCtx context.Context) error {
		return uc.publisher.ConfirmationSent(txCtx, events.NotificationConfirmationSent{
			SagaID: in.SagaID,
			Email:  in.Email,
		})
	}); err != nil {
		return fmt.Errorf("publish confirmation sent: %w", err)
	}

	uc.publisher.Notify()

	return nil
}
