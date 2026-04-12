package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github-release-notifier/internal/domain"
)

var repoRegex = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

type RepoRepository interface {
	Create(ctx context.Context, repo *domain.Repository) error
	GetByName(ctx context.Context, name string) (*domain.Repository, error)
}

type SubscriptionRepository interface {
	Create(ctx context.Context, sub *domain.Subscription) error
	GetByEmail(ctx context.Context, email string) ([]*domain.SubscriptionView, error)
	Confirm(ctx context.Context, token string) error
	Delete(ctx context.Context, token string) error
}

type GitHubClient interface {
	RepoExists(ctx context.Context, owner, repo string) (bool, error)
}

type Mailer interface {
	SendConfirmation(to, repo, confirmURL string) error
}

type SubscriptionService struct {
	repos   RepoRepository
	subs    SubscriptionRepository
	github  GitHubClient
	mailer  Mailer
	baseURL string
}

func NewSubscriptionService(
	repos RepoRepository,
	subs SubscriptionRepository,
	github GitHubClient,
	mailer Mailer,
	baseURL string,
) *SubscriptionService {
	return &SubscriptionService{
		repos:   repos,
		subs:    subs,
		github:  github,
		mailer:  mailer,
		baseURL: baseURL,
	}
}

func (s *SubscriptionService) Subscribe(ctx context.Context, email, repoName string) error {
	owner, name, err := parseRepo(repoName)
	if err != nil {
		return err
	}

	exists, err := s.github.RepoExists(ctx, owner, name)
	if err != nil {
		return fmt.Errorf("check repo exists: %w", err)
	}

	if !exists {
		return ErrRepoNotFound
	}

	repo, err := s.repos.GetByName(ctx, repoName)
	if errors.Is(err, domain.ErrNotFound) {
		repo = &domain.Repository{Name: repoName}
		if err := s.repos.Create(ctx, repo); err != nil {
			return fmt.Errorf("create repository: %w", err)
		}

	} else if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	confirmToken, err := randomToken()
	if err != nil {
		return fmt.Errorf("generate confirm token: %w", err)
	}

	unsubscribeToken, err := randomToken()
	if err != nil {
		return fmt.Errorf("generate unsubscribe token: %w", err)
	}

	sub := &domain.Subscription{
		RepositoryID:     repo.ID,
		Email:            email,
		ConfirmToken:     confirmToken,
		UnsubscribeToken: unsubscribeToken,
	}

	if err := s.subs.Create(ctx, sub); err != nil {
		return err
	}

	confirmURL := fmt.Sprintf("%s/api/confirm/%s", s.baseURL, confirmToken)
	go func() {
		if err := s.mailer.SendConfirmation(email, repoName, confirmURL); err != nil {
			slog.Error("failed to send confirmation email", "email", email, "error", err)
		}
	}()

	return nil
}

func (s *SubscriptionService) Confirm(ctx context.Context, token string) error {
	return s.subs.Confirm(ctx, token)
}

func (s *SubscriptionService) Unsubscribe(ctx context.Context, token string) error {
	return s.subs.Delete(ctx, token)
}

func (s *SubscriptionService) GetByEmail(ctx context.Context, email string) ([]*domain.SubscriptionView, error) {
	return s.subs.GetByEmail(ctx, email)
}

func parseRepo(repo string) (owner, name string, err error) {
	if !repoRegex.MatchString(repo) {
		return "", "", ErrInvalidRepo
	}

	parts := strings.SplitN(repo, "/", 2)
	return parts[0], parts[1], nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}