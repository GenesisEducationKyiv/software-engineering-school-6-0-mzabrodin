package unsubscribe

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/subscription/domain"
)

type subRepository interface {
	Delete(ctx context.Context, token string) (domain.RemovedSubscription, error)
}

type transactor interface {
	Within(ctx context.Context, fn func(ctx context.Context) error) error
}

type publisher interface {
	SubscriptionRemoved(ctx context.Context, ev events.SubscriptionRemoved) error
	Notify()
}

type Input struct {
	Token string
}

type Output struct{}

type UseCase struct {
	subs subRepository
	tx   transactor
	pub  publisher
	log  *slog.Logger
}

func New(subs subRepository, tx transactor, pub publisher, log *slog.Logger) *UseCase {
	return &UseCase{subs: subs, tx: tx, pub: pub, log: log.With("component", "unsubscribe")}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	err := uc.tx.Within(ctx, func(ctx context.Context) error {
		removed, err := uc.subs.Delete(ctx, in.Token)
		if err != nil {
			return err
		}

		return uc.pub.SubscriptionRemoved(ctx, events.SubscriptionRemoved{
			SagaID:   uuid.NewString(),
			Email:    removed.Email,
			RepoName: removed.Repo,
		})
	})
	if err != nil {
		return Output{}, err
	}

	uc.pub.Notify()
	uc.log.InfoContext(ctx, "subscription deleted")

	return Output{}, nil
}
