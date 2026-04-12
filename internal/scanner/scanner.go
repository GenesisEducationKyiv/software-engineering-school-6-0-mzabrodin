package scanner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github-release-notifier/internal/domain"

	"github.com/google/uuid"
)

type Mailer interface {
	SendReleaseNotifications(notifications []domain.ReleaseNotification) error
}

type GitHubClient interface {
	GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error)
}

type RepoRepository interface {
	GetAllWithSubscriptions(ctx context.Context) ([]*domain.Repository, error)
	UpdateLastSeenTag(ctx context.Context, name string, tag string) error
}

type SubscriptionRepository interface {
	GetConfirmedByRepoID(ctx context.Context, repoID uuid.UUID) ([]*domain.Subscription, error)
}

type Scanner struct {
	repos    RepoRepository
	subs     SubscriptionRepository
	github   GitHubClient
	mailer   Mailer
	interval time.Duration
	baseURL  string
}

func NewScanner(
	repos RepoRepository,
	subs SubscriptionRepository,
	gh GitHubClient,
	mailer Mailer,
	interval time.Duration,
	baseURL string,
) *Scanner {
	return &Scanner{
		repos:    repos,
		subs:     subs,
		github:   gh,
		mailer:   mailer,
		interval: interval,
		baseURL:  baseURL,
	}
}

func (s *Scanner) Start(ctx context.Context) {
	slog.Info("scanner started", "interval", s.interval)

	s.scan(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("scanner stopped")
			return
		case <-ticker.C:
			s.scan(ctx)
		}
	}
}

func (s *Scanner) scan(ctx context.Context) {
	slog.Info("scanning repositories for new releases")

	repos, err := s.repos.GetAllWithSubscriptions(ctx)
	if err != nil {
		slog.Error("failed to get repositories", "error", err)
		return
	}

	for _, repo := range repos {
		if err := s.checkRepo(ctx, repo); err != nil {
			slog.Error("failed to check repository", "repo", repo.Name, "error", err)
		}
	}
}

func (s *Scanner) checkRepo(ctx context.Context, repo *domain.Repository) error {
	owner, name, err := splitRepo(repo.Name)
	if err != nil {
		return err
	}

	release, err := s.github.GetLatestRelease(ctx, owner, name)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			slog.Warn("GitHub token is invalid or missing, skipping scan", "repo", repo.Name)
			return nil
		}

		if errors.Is(err, domain.ErrRateLimited) {
			slog.Warn("rate limited by GitHub, skipping scan", "repo", repo.Name)
			return nil
		}

		if errors.Is(err, domain.ErrNoRelease) {
			return nil
		}

		return fmt.Errorf("get latest release: %w", err)
	}

	if repo.LastSeenTag != nil && *repo.LastSeenTag == release.TagName {
		return nil
	}

	slog.Info("new release detected", "repo", repo.Name, "tag", release.TagName)

	subs, err := s.subs.GetConfirmedByRepoID(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("get subscribers: %w", err)
	}

	notifications := make([]domain.ReleaseNotification, 0, len(subs))
	for _, sub := range subs {
		notifications = append(notifications, domain.ReleaseNotification{
			To:             sub.Email,
			Repo:           repo.Name,
			Tag:            release.TagName,
			ReleaseURL:     release.HTMLURL,
			UnsubscribeURL: fmt.Sprintf("%s/api/unsubscribe/%s", s.baseURL, sub.UnsubscribeToken),
		})
	}

	if err := s.mailer.SendReleaseNotifications(notifications); err != nil {
		return fmt.Errorf("send notifications: %w", err)
	}

	if err := s.repos.UpdateLastSeenTag(ctx, repo.Name, release.TagName); err != nil {
		return fmt.Errorf("update last seen tag: %w", err)
	}

	slog.Info("notifications sent", "repo", repo.Name, "tag", release.TagName, "count", len(notifications))

	return nil
}

func splitRepo(repo string) (owner, name string, err error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo format: %s", repo)
	}

	return parts[0], parts[1], nil
}
