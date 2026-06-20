package retry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/domain"
	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/events"
)

const (
	releaseDeadReason         = "release email delivery failed after max retries"
	confirmationDeadReason    = "confirmation email delivery failed after max retries"
	confirmationExpiredReason = "confirmation link expired before delivery"
)

type failedNotificationStore interface {
	ListRetryable(ctx context.Context, maxRetries int) ([]domain.FailedNotification, error)
	IncrementRetry(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
}

type failedConfirmationStore interface {
	ListRetryable(ctx context.Context, maxRetries int, notBefore time.Time) ([]domain.FailedConfirmation, error)
	ListExpired(ctx context.Context, cutoff time.Time) ([]domain.FailedConfirmation, error)
	IncrementRetry(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
}

type recipientResolver interface {
	Recipient(ctx context.Context, email, repoName string) (domain.Recipient, error)
}

type releaseSender interface {
	SendReleaseNotifications(ctx context.Context, notifications []notifier.ReleaseNotification) notifier.BatchResult
}

type confirmationSender interface {
	DeliverConfirmation(ctx context.Context, to, repo, confirmURL string) error
}

type urlBuilder interface {
	UnsubscribeURL(token string) string
}

type publisher interface {
	ReleaseSent(ctx context.Context, ev events.NotificationReleaseSent) error
	ReleaseDead(ctx context.Context, ev events.NotificationReleaseDead) error
	ConfirmationSent(ctx context.Context, ev events.NotificationConfirmationSent) error
	ConfirmationDead(ctx context.Context, ev events.NotificationConfirmationDead) error
}

type Config struct {
	MaxRetries      int
	ConfirmationTTL time.Duration
}

type Retrier struct {
	notifications failedNotificationStore
	confirmations failedConfirmationStore
	recipients    recipientResolver
	relSender     releaseSender
	confSender    confirmationSender
	urls          urlBuilder
	publisher     publisher
	cfg           Config
	log           *slog.Logger
}

func New(
	notifications failedNotificationStore,
	confirmations failedConfirmationStore,
	recipients recipientResolver,
	relSender releaseSender,
	confSender confirmationSender,
	urls urlBuilder,
	pub publisher,
	cfg Config,
	log *slog.Logger,
) *Retrier {
	return &Retrier{
		notifications: notifications,
		confirmations: confirmations,
		recipients:    recipients,
		relSender:     relSender,
		confSender:    confSender,
		urls:          urls,
		publisher:     pub,
		cfg:           cfg,
		log:           log.With("component", "notifier-retry"),
	}
}

func (r *Retrier) Releases(ctx context.Context) error {
	rows, err := r.notifications.ListRetryable(ctx, r.cfg.MaxRetries)
	if err != nil {
		return fmt.Errorf("list retryable notifications: %w", err)
	}

	if len(rows) > 0 {
		r.log.InfoContext(ctx, "retrying failed release notifications", "count", len(rows))
	}

	for i := range rows {
		r.retryRelease(ctx, &rows[i])
	}

	return nil
}

func (r *Retrier) Confirmations(ctx context.Context) error {
	cutoff := time.Now().Add(-r.cfg.ConfirmationTTL)

	expired, err := r.confirmations.ListExpired(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("list expired confirmations: %w", err)
	}

	if len(expired) > 0 {
		r.log.InfoContext(ctx, "dead-lettering expired confirmations", "count", len(expired))
	}

	for i := range expired {
		r.deadLetterConfirmation(ctx, &expired[i], confirmationExpiredReason)
	}

	rows, err := r.confirmations.ListRetryable(ctx, r.cfg.MaxRetries, cutoff)
	if err != nil {
		return fmt.Errorf("list retryable confirmations: %w", err)
	}

	if len(rows) > 0 {
		r.log.InfoContext(ctx, "retrying failed confirmations", "count", len(rows))
	}

	for i := range rows {
		r.retryConfirmation(ctx, &rows[i])
	}

	return nil
}

func (r *Retrier) retryRelease(ctx context.Context, fn *domain.FailedNotification) {
	rec, err := r.recipients.Recipient(ctx, fn.Email, fn.RepoName)
	if errors.Is(err, entity.ErrNotFound) {
		r.deleteNotification(ctx, fn.ID)

		return
	}

	if err != nil {
		r.log.ErrorContext(ctx, "resolve recipient for release retry failed",
			"email", fn.Email, "repo", fn.RepoName, "error", err)

		return
	}

	result := r.relSender.SendReleaseNotifications(ctx, []notifier.ReleaseNotification{{
		To:             fn.Email,
		Repo:           fn.RepoName,
		Tag:            fn.Tag,
		ReleaseURL:     fn.ReleaseURL,
		UnsubscribeURL: r.urls.UnsubscribeURL(rec.UnsubToken),
	}})

	if result.Sent > 0 {
		r.deleteNotification(ctx, fn.ID)
		notifier.TryPublish(ctx, r.log, "release sent", func() error {
			return r.publisher.ReleaseSent(ctx, events.NotificationReleaseSent{
				SagaID: fn.SagaID, RepoName: fn.RepoName, Tag: fn.Tag, Email: fn.Email,
			})
		})

		return
	}

	if fn.RetryCount+1 >= r.cfg.MaxRetries {
		r.deleteNotification(ctx, fn.ID)
		notifier.TryPublish(ctx, r.log, "release dead", func() error {
			return r.publisher.ReleaseDead(ctx, events.NotificationReleaseDead{
				SagaID: fn.SagaID, RepoName: fn.RepoName, Tag: fn.Tag, Email: fn.Email, Reason: releaseDeadReason,
			})
		})

		return
	}

	if err := r.notifications.IncrementRetry(ctx, fn.ID); err != nil {
		r.log.ErrorContext(ctx, "increment notification retry failed", "id", fn.ID, "error", err)
	}
}

func (r *Retrier) retryConfirmation(ctx context.Context, fc *domain.FailedConfirmation) {
	if err := r.confSender.DeliverConfirmation(ctx, fc.Email, fc.RepoName, fc.ConfirmURL); err != nil {
		r.log.WarnContext(ctx, "confirmation retry failed", "email", fc.Email, "repo", fc.RepoName, "error", err)

		if fc.RetryCount+1 >= r.cfg.MaxRetries {
			r.deadLetterConfirmation(ctx, fc, confirmationDeadReason)

			return
		}

		if incErr := r.confirmations.IncrementRetry(ctx, fc.ID); incErr != nil {
			r.log.ErrorContext(ctx, "increment confirmation retry failed", "id", fc.ID, "error", incErr)
		}

		return
	}

	r.deleteConfirmation(ctx, fc.ID)
	notifier.TryPublish(ctx, r.log, "confirmation sent", func() error {
		return r.publisher.ConfirmationSent(
			ctx,
			events.NotificationConfirmationSent{SagaID: fc.SagaID, Email: fc.Email},
		)
	})
}

func (r *Retrier) deadLetterConfirmation(ctx context.Context, fc *domain.FailedConfirmation, reason string) {
	r.deleteConfirmation(ctx, fc.ID)
	notifier.TryPublish(ctx, r.log, "confirmation dead", func() error {
		return r.publisher.ConfirmationDead(ctx, events.NotificationConfirmationDead{
			SagaID: fc.SagaID, Email: fc.Email, Reason: reason,
		})
	})
}

func (r *Retrier) deleteNotification(ctx context.Context, id int64) {
	if err := r.notifications.Delete(ctx, id); err != nil {
		r.log.ErrorContext(ctx, "delete failed_notification failed", "id", id, "error", err)
	}
}

func (r *Retrier) deleteConfirmation(ctx context.Context, id int64) {
	if err := r.confirmations.Delete(ctx, id); err != nil {
		r.log.ErrorContext(ctx, "delete failed_confirmation failed", "id", id, "error", err)
	}
}
