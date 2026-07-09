package confirm

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/subscription/domain"
)

type subRepository interface {
	Confirm(ctx context.Context, email, repo string) (domain.ConfirmResult, error)
}

type tokenVerifier interface {
	Verify(token string) (email, repo string, err error)
}

type transactor interface {
	Within(ctx context.Context, fn func(ctx context.Context) error) error
}

type publisher interface {
	SubscriptionConfirmed(ctx context.Context, ev events.SubscriptionConfirmed) error
	Notify()
}

type Input struct {
	Token string
}

type Output struct{}

type UseCase struct {
	subs   subRepository
	tokens tokenVerifier
	tx     transactor
	pub    publisher
	log    *slog.Logger
}

func New(subs subRepository, tokens tokenVerifier, tx transactor, pub publisher, log *slog.Logger) *UseCase {
	return &UseCase{subs: subs, tokens: tokens, tx: tx, pub: pub, log: log.With("component", "confirm")}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	email, repo, err := uc.tokens.Verify(in.Token)
	if err != nil {
		return Output{}, err
	}

	var result domain.ConfirmResult

	err = uc.tx.Within(ctx, func(ctx context.Context) error {
		result, err = uc.subs.Confirm(ctx, email, repo)
		if err != nil {
			return err
		}

		if !result.Confirmed {
			uc.log.DebugContext(ctx, "subscription already confirmed or not found", "email", email, "repo", repo)
			return nil
		}

		return uc.pub.SubscriptionConfirmed(ctx, events.SubscriptionConfirmed{
			SagaID:     uuid.NewString(),
			Email:      email,
			RepoName:   repo,
			UnsubToken: result.UnsubToken,
		})
	})
	if err != nil {
		return Output{}, err
	}

	if result.Confirmed {
		uc.pub.Notify()
		uc.log.InfoContext(ctx, "subscription confirmed", "email", email, "repo", repo)
	}

	return Output{}, nil
}
