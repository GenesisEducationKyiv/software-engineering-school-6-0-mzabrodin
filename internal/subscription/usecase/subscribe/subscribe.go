package subscribe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/github"
)

type repoRepository interface {
	Create(ctx context.Context, repo *entity.Repository) error
	GetByName(ctx context.Context, name string) (*entity.Repository, error)
}

type subRepository interface {
	Create(ctx context.Context, sub *entity.Subscription) error
}

type gitHubClient interface {
	RepoExists(ctx context.Context, owner, repo string) (bool, error)
}

type mailer interface {
	SendConfirmation(ctx context.Context, to, repo, confirmURL string)
}

type urlBuilder interface {
	ConfirmURL(token string) string
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
	mailer mailer
	urls   urlBuilder
	log    *slog.Logger
}

func New(
	repos repoRepository,
	subs subRepository,
	gh gitHubClient,
	mailer mailer,
	urls urlBuilder,
	log *slog.Logger,
) *UseCase {
	return &UseCase{
		repos:  repos,
		subs:   subs,
		gh:     gh,
		mailer: mailer,
		urls:   urls,
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

	repo, err := uc.ensureRepoStored(ctx, in.Repo)
	if err != nil {
		return Output{}, err
	}

	confirmToken, err := uc.createSubscription(ctx, in.Email, repo)
	if err != nil {
		return Output{}, err
	}

	uc.log.InfoContext(ctx, "subscription created", "email", in.Email, "repo", in.Repo)
	uc.mailer.SendConfirmation(ctx, in.Email, in.Repo, uc.urls.ConfirmURL(confirmToken))

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

func (uc *UseCase) ensureRepoStored(ctx context.Context, repoName string) (*entity.Repository, error) {
	repo, err := uc.repos.GetByName(ctx, repoName)
	if err == nil {
		return repo, nil
	}

	if !errors.Is(err, entity.ErrNotFound) {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	repo = entity.NewRepository(repoName)
	if err := uc.repos.Create(ctx, repo); err != nil {
		return nil, fmt.Errorf("create repository: %w", err)
	}

	uc.log.InfoContext(ctx, "repository tracked", "repo", repoName)

	return repo, nil
}

func (uc *UseCase) createSubscription(
	ctx context.Context,
	email string,
	repo *entity.Repository,
) (string, error) {
	sub, err := entity.NewSubscription(repo.ID, email)
	if err != nil {
		return "", fmt.Errorf("new subscription: %w", err)
	}

	if err := uc.subs.Create(ctx, sub); err != nil {
		return "", fmt.Errorf("create subscription: %w", err)
	}

	return sub.ConfirmToken, nil
}
