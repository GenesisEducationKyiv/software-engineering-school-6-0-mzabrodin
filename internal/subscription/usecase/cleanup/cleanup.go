package cleanup

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/subscription/domain"
)

type subRepository interface {
	DeleteExpiredPending(ctx context.Context, cutoff time.Time) ([]domain.ExpiredSubscription, error)
}

type transactor interface {
	Within(ctx context.Context, fn func(ctx context.Context) error) error
}

type publisher interface {
	SubscriptionExpired(ctx context.Context, ev events.SubscriptionExpired) error
	Notify()
}

type UseCase struct {
	subs   subRepository
	tx     transactor
	pub    publisher
	maxAge time.Duration
	log    *slog.Logger
}

func New(subs subRepository, tx transactor, pub publisher, maxAge time.Duration, log *slog.Logger) *UseCase {
	return &UseCase{subs: subs, tx: tx, pub: pub, maxAge: maxAge, log: log.With("component", "cleanup")}
}

func (uc *UseCase) Run(ctx context.Context) error {
	cutoff := time.Now().Add(-uc.maxAge)

	var expired []domain.ExpiredSubscription

	if err := uc.tx.Within(ctx, func(ctx context.Context) error {
		var err error

		expired, err = uc.subs.DeleteExpiredPending(ctx, cutoff)
		if err != nil {
			return err
		}

		for _, s := range expired {
			if err := uc.pub.SubscriptionExpired(ctx, events.SubscriptionExpired{
				SagaID:   uuid.NewString(),
				Email:    s.Email,
				RepoName: s.Repo,
			}); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("clean up expired pending subscriptions: %w", err)
	}

	if len(expired) > 0 {
		uc.pub.Notify()
		uc.log.InfoContext(ctx, "expired pending subscriptions removed", "count", len(expired))
	}

	return nil
}
