package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"

	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/infrastructure/metrics"
)

// 32 random bytes encoded as 64-character hex string
const tokenBytes = 32

type gitHubRepoRepository interface {
	Create(ctx context.Context, repo *entity.Repository) error
	GetByName(ctx context.Context, name string) (*entity.Repository, error)
}

type subscriptionRepository interface {
	Create(ctx context.Context, sub *entity.Subscription) error
	GetByEmail(ctx context.Context, email string) ([]*entity.SubscriptionView, error)
	Confirm(ctx context.Context, token string) error
	Delete(ctx context.Context, token string) error
}

type gitHubClient interface {
	RepoExists(ctx context.Context, owner, repo string) (bool, error)
}

type mailer interface {
	SendConfirmation(ctx context.Context, to, repo, confirmURL string)
	Shutdown()
}

type urlBuilder interface {
	ConfirmURL(token string) string
}

type SubscriptionService struct {
	repos  gitHubRepoRepository
	subs   subscriptionRepository
	github gitHubClient
	mailer mailer
	urls   urlBuilder
	log    *slog.Logger
}

func NewSubscriptionService(
	repos gitHubRepoRepository,
	subs subscriptionRepository,
	github gitHubClient,
	mailer mailer,
	urls urlBuilder,
	log *slog.Logger,
) *SubscriptionService {
	return &SubscriptionService{
		repos:  repos,
		subs:   subs,
		github: github,
		mailer: mailer,
		urls:   urls,
		log:    log.With("component", "service"),
	}
}

func (s *SubscriptionService) Subscribe(ctx context.Context, email, repoName string) (err error) {
	defer func() {
		metrics.SubscriptionOperationsTotal.WithLabelValues("subscribe", metrics.ResultLabel(err)).Inc()
	}()

	if _, err := mail.ParseAddress(email); err != nil {
		return entity.ErrInvalidEmail
	}

	owner, name, err := entity.ParseRepo(repoName)
	if err != nil {
		return err
	}

	if err := s.ensureRepoExists(ctx, owner, name); err != nil {
		return err
	}

	repo, err := s.ensureRepoStored(ctx, repoName)
	if err != nil {
		return err
	}

	confirmToken, err := s.createSubscription(ctx, email, repo)
	if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "subscription created", "email", email, "repo", repoName)
	s.mailer.SendConfirmation(ctx, email, repoName, s.urls.ConfirmURL(confirmToken))

	return nil
}

func (s *SubscriptionService) ensureRepoExists(ctx context.Context, owner, name string) error {
	exists, err := s.github.RepoExists(ctx, owner, name)
	if err != nil {
		return fmt.Errorf("check repo exists: %w", err)
	}

	if !exists {
		return entity.ErrRepoNotFound
	}

	return nil
}

func (s *SubscriptionService) ensureRepoStored(ctx context.Context, repoName string) (*entity.Repository, error) {
	repo, err := s.repos.GetByName(ctx, repoName)
	if err == nil {
		return repo, nil
	}

	if !errors.Is(err, entity.ErrNotFound) {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	repo = &entity.Repository{Name: repoName}
	if err := s.repos.Create(ctx, repo); err != nil {
		return nil, fmt.Errorf("create repository: %w", err)
	}

	s.log.InfoContext(ctx, "repository tracked", "repo", repoName)

	return repo, nil
}

func (s *SubscriptionService) createSubscription(
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

	if err := s.subs.Create(ctx, sub); err != nil {
		return "", fmt.Errorf("create subscription: %w", err)
	}

	return confirmToken, nil
}

func (s *SubscriptionService) Confirm(ctx context.Context, token string) (err error) {
	defer func() {
		metrics.SubscriptionOperationsTotal.WithLabelValues("confirm", metrics.ResultLabel(err)).Inc()
	}()

	if err := s.subs.Confirm(ctx, token); err != nil {
		return err
	}
	s.log.InfoContext(ctx, "subscription confirmed")
	return nil
}

func (s *SubscriptionService) Unsubscribe(ctx context.Context, token string) (err error) {
	defer func() {
		metrics.SubscriptionOperationsTotal.WithLabelValues("unsubscribe", metrics.ResultLabel(err)).Inc()
	}()

	if err := s.subs.Delete(ctx, token); err != nil {
		return err
	}
	s.log.InfoContext(ctx, "subscription deleted")
	return nil
}

func (s *SubscriptionService) Shutdown() {
	s.mailer.Shutdown()
}

func (s *SubscriptionService) GetByEmail(
	ctx context.Context,
	email string,
) (views []*entity.SubscriptionView, err error) {
	defer func() {
		metrics.SubscriptionOperationsTotal.WithLabelValues("list", metrics.ResultLabel(err)).Inc()
	}()

	if _, parseErr := mail.ParseAddress(email); parseErr != nil {
		return nil, entity.ErrInvalidEmail
	}

	return s.subs.GetByEmail(ctx, email)
}

func randomToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}

	return hex.EncodeToString(b), nil
}
