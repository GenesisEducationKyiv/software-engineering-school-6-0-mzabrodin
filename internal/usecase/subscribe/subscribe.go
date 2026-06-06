package subscribe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"

	"github-release-notifier/internal/entity"
)

// 32 random bytes encoded as 64-character hex string
const tokenBytes = 32

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
	github gitHubClient
	mailer mailer
	urls   urlBuilder
	log    *slog.Logger
}

func New(
	repos repoRepository,
	subs subRepository,
	github gitHubClient,
	mailer mailer,
	urls urlBuilder,
	log *slog.Logger,
) *UseCase {
	return &UseCase{
		repos:  repos,
		subs:   subs,
		github: github,
		mailer: mailer,
		urls:   urls,
		log:    log.With("component", "subscribe"),
	}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	if _, err := mail.ParseAddress(in.Email); err != nil {
		return Output{}, entity.ErrInvalidEmail
	}

	owner, name, err := entity.ParseRepo(in.Repo)
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
	exists, err := uc.github.RepoExists(ctx, owner, name)
	if err != nil {
		return fmt.Errorf("check repo exists: %w", err)
	}

	if !exists {
		return entity.ErrRepoNotFound
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

	repo = &entity.Repository{Name: repoName}
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
	confirmToken, err := randomToken()
	if err != nil {
		return "", fmt.Errorf("generate confirm token: %w", err)
	}

	unsubscribeToken, err := randomToken()
	if err != nil {
		return "", fmt.Errorf("generate unsubscribe token: %w", err)
	}

	sub := &entity.Subscription{
		RepositoryID:     repo.ID,
		Email:            email,
		ConfirmToken:     confirmToken,
		UnsubscribeToken: unsubscribeToken,
	}

	if err := uc.subs.Create(ctx, sub); err != nil {
		return "", fmt.Errorf("create subscription: %w", err)
	}

	return confirmToken, nil
}

func randomToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}

	return hex.EncodeToString(b), nil
}
