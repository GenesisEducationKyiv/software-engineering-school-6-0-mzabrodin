package subscribe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	shareddomain "github-release-notifier/internal/shared/domain"
	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/shared/github"
	"github-release-notifier/internal/subscription/domain"
)

type repoRepository interface {
	GetOrCreate(ctx context.Context, name string) (domain.Repository, error)
}

type subRepository interface {
	Create(ctx context.Context, sub domain.Subscription) error
	FindByEmailAndRepo(ctx context.Context, email string, repoID uuid.UUID) (domain.Subscription, error)
}

type gitHubClient interface {
	RepoExists(ctx context.Context, owner, repo string) (bool, error)
}

type tokenIssuer interface {
	Issue(email, repo string) (string, error)
}

type urlBuilder interface {
	ConfirmURL(token string) string
}

type transactor interface {
	Within(ctx context.Context, fn func(ctx context.Context) error) error
}

type publisher interface {
	SubscriptionPending(ctx context.Context, ev events.SubscriptionPending) error
	Notify()
}

type Input struct {
	Email string
	Repo  string
}

type Output struct{}

type UseCase struct {
	repos  repoRepository
	subs   subRepository
	gh     gitHubClient
	tokens tokenIssuer
	urls   urlBuilder
	tx     transactor
	pub    publisher
	log    *slog.Logger
}

func New(
	repos repoRepository,
	subs subRepository,
	gh gitHubClient,
	tokens tokenIssuer,
	urls urlBuilder,
	tx transactor,
	pub publisher,
	log *slog.Logger,
) *UseCase {
	return &UseCase{
		repos:  repos,
		subs:   subs,
		gh:     gh,
		tokens: tokens,
		urls:   urls,
		tx:     tx,
		pub:    pub,
		log:    log.With("component", "subscribe"),
	}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	owner, name, err := github.ParseRepo(in.Repo)
	if err != nil {
		return Output{}, err
	}

	if err := uc.ensureRepoExists(ctx, owner, name); err != nil {
		return Output{}, err
	}

	repo, err := uc.repos.GetOrCreate(ctx, in.Repo)
	if err != nil {
		return Output{}, fmt.Errorf("ensure repository: %w", err)
	}

	isNew, err := uc.resolveExisting(ctx, in.Email, repo.ID)
	if err != nil {
		return Output{}, err
	}

	return uc.emitPending(ctx, in.Email, in.Repo, repo.ID, isNew)
}

func (uc *UseCase) resolveExisting(ctx context.Context, email string, repoID uuid.UUID) (bool, error) {
	existing, err := uc.subs.FindByEmailAndRepo(ctx, email, repoID)
	switch {
	case err == nil && existing.Confirmed:
		return false, shareddomain.ErrAlreadyExists
	case err == nil:
		return false, nil
	case errors.Is(err, shareddomain.ErrNotFound):
		return true, nil
	default:
		return false, fmt.Errorf("find subscription: %w", err)
	}
}

func (uc *UseCase) emitPending(ctx context.Context, email, repo string, repoID uuid.UUID, isNew bool) (Output, error) {
	token, err := uc.tokens.Issue(email, repo)
	if err != nil {
		return Output{}, fmt.Errorf("issue confirmation token: %w", err)
	}

	var sub domain.Subscription
	if isNew {
		if sub, err = domain.NewSubscription(repoID, email); err != nil {
			return Output{}, fmt.Errorf("new subscription: %w", err)
		}
	}

	ev := events.SubscriptionPending{
		SagaID:     uuid.NewString(),
		Email:      email,
		RepoName:   repo,
		ConfirmURL: uc.urls.ConfirmURL(token),
	}

	err = uc.tx.Within(ctx, func(ctx context.Context) error {
		if isNew {
			if err := uc.subs.Create(ctx, sub); err != nil {
				return err
			}
		}

		return uc.pub.SubscriptionPending(ctx, ev)
	})
	if err != nil {
		return Output{}, err
	}

	uc.pub.Notify()
	uc.log.InfoContext(ctx, "subscription pending", "email", email, "repo", repo, "resubscribe", !isNew)

	return Output{}, nil
}

func (uc *UseCase) ensureRepoExists(ctx context.Context, owner, name string) error {
	exists, err := uc.gh.RepoExists(ctx, owner, name)
	if err != nil {
		return fmt.Errorf("check repo exists: %w", err)
	}

	if !exists {
		return github.ErrRepoNotFound
	}

	return nil
}
