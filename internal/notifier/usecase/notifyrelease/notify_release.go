package notifyrelease

import (
	"context"
	"fmt"
	"log/slog"

	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/domain"
	"github-release-notifier/internal/shared/events"
)

const failureReason = "release email delivery failed"

type recipientLister interface {
	RecipientsByRepo(ctx context.Context, repoName string) ([]domain.Recipient, error)
}

type dedupeStore interface {
	Exists(ctx context.Context, repoName, tag string) (bool, error)
	Mark(ctx context.Context, repoName, tag string) error
}

type failedStore interface {
	Add(ctx context.Context, fn *domain.FailedNotification) error
}

type sender interface {
	SendReleaseNotifications(ctx context.Context, notifications []notifier.ReleaseNotification) notifier.BatchResult
}

type urlBuilder interface {
	UnsubscribeURL(token string) string
}

type transactor interface {
	Within(ctx context.Context, fn func(ctx context.Context) error) error
}

type publisher interface {
	ReleaseSent(ctx context.Context, ev events.NotificationReleaseSent) error
	ReleaseFailed(ctx context.Context, ev events.NotificationReleaseFailed) error
	ReleaseNotified(ctx context.Context, ev events.ReleaseNotified) error
	Notify()
}

type Input struct {
	SagaID     string
	RepoName   string
	Tag        string
	ReleaseURL string
}

type Output struct {
	SentCount    int
	FailedEmails []string
}

type UseCase struct {
	recipients recipientLister
	dedupe     dedupeStore
	failed     failedStore
	sender     sender
	urls       urlBuilder
	tx         transactor
	publisher  publisher
	log        *slog.Logger
}

func New(
	recipients recipientLister,
	dedupe dedupeStore,
	failed failedStore,
	mailSender sender,
	urls urlBuilder,
	tx transactor,
	pub publisher,
	log *slog.Logger,
) *UseCase {
	return &UseCase{
		recipients: recipients,
		dedupe:     dedupe,
		failed:     failed,
		sender:     mailSender,
		urls:       urls,
		tx:         tx,
		publisher:  pub,
		log:        log.With("component", "notify-release"),
	}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	uc.log.InfoContext(ctx, "processing detected release", "repo", in.RepoName, "tag", in.Tag)

	processed, err := uc.dedupe.Exists(ctx, in.RepoName, in.Tag)
	if err != nil {
		return Output{}, fmt.Errorf("check processed release: %w", err)
	}

	if processed {
		uc.log.InfoContext(ctx, "release already processed; skipping", "repo", in.RepoName, "tag", in.Tag)

		return Output{}, nil
	}

	recipients, err := uc.recipients.RecipientsByRepo(ctx, in.RepoName)
	if err != nil {
		return Output{}, fmt.Errorf("resolve recipients: %w", err)
	}

	uc.log.InfoContext(ctx, "delivering release notifications",
		"repo", in.RepoName, "tag", in.Tag, "recipients", len(recipients))

	result := uc.deliver(ctx, in, recipients)

	if err := uc.tx.Within(ctx, func(txCtx context.Context) error {
		if err := uc.recordOutcomes(txCtx, in, recipients, result); err != nil {
			return err
		}

		if err := uc.publisher.ReleaseNotified(txCtx, events.ReleaseNotified{
			SagaID:       in.SagaID,
			RepoName:     in.RepoName,
			Tag:          in.Tag,
			SentCount:    result.Sent,
			FailedEmails: result.Failed,
		}); err != nil {
			return err
		}

		return uc.dedupe.Mark(txCtx, in.RepoName, in.Tag)
	}); err != nil {
		return Output{}, fmt.Errorf("commit release outcome: %w", err)
	}

	uc.publisher.Notify()
	uc.log.InfoContext(ctx, "release notifications processed",
		"repo", in.RepoName, "tag", in.Tag, "sent", result.Sent, "failed", len(result.Failed))

	return Output{SentCount: result.Sent, FailedEmails: result.Failed}, nil
}

func (uc *UseCase) deliver(
	ctx context.Context,
	in Input,
	recipients []domain.Recipient,
) notifier.BatchResult {
	if len(recipients) == 0 {
		return notifier.BatchResult{}
	}

	notifications := make([]notifier.ReleaseNotification, 0, len(recipients))
	for _, r := range recipients {
		notifications = append(notifications, notifier.ReleaseNotification{
			To:             r.Email,
			Repo:           in.RepoName,
			Tag:            in.Tag,
			ReleaseURL:     in.ReleaseURL,
			UnsubscribeURL: uc.urls.UnsubscribeURL(r.UnsubToken),
		})
	}

	return uc.sender.SendReleaseNotifications(ctx, notifications)
}

func (uc *UseCase) recordOutcomes(
	ctx context.Context,
	in Input,
	recipients []domain.Recipient,
	result notifier.BatchResult,
) error {
	failed := make(map[string]bool, len(result.Failed))
	for _, email := range result.Failed {
		failed[email] = true
	}

	for _, r := range recipients {
		if failed[r.Email] {
			if err := uc.failed.Add(ctx, &domain.FailedNotification{
				SagaID:     in.SagaID,
				RepoName:   in.RepoName,
				Tag:        in.Tag,
				ReleaseURL: in.ReleaseURL,
				Email:      r.Email,
				Reason:     failureReason,
			}); err != nil {
				return err
			}

			if err := uc.publisher.ReleaseFailed(ctx, events.NotificationReleaseFailed{
				SagaID:   in.SagaID,
				RepoName: in.RepoName,
				Tag:      in.Tag,
				Email:    r.Email,
				Reason:   failureReason,
			}); err != nil {
				return err
			}

			continue
		}

		if err := uc.publisher.ReleaseSent(ctx, events.NotificationReleaseSent{
			SagaID:   in.SagaID,
			RepoName: in.RepoName,
			Tag:      in.Tag,
			Email:    r.Email,
		}); err != nil {
			return err
		}
	}

	return nil
}
