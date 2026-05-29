package scanner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github-release-notifier/internal/domain"
	"github-release-notifier/internal/metrics"
)

type notifier interface {
	Notify(ctx context.Context, subs []*domain.Subscription, repo *domain.Repository, release *domain.Release) error
}

type gitHubClient interface {
	GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error)
}

type gitHubRepoRepository interface {
	GetAllWithSubscriptions(ctx context.Context) ([]*domain.Repository, error)
	UpdateLastSeenTag(ctx context.Context, name string, tag string) error
}

type subscriptionRepository interface {
	GetConfirmedByRepoID(ctx context.Context, repoID uuid.UUID) ([]*domain.Subscription, error)
}

type Scanner struct {
	repos    gitHubRepoRepository
	subs     subscriptionRepository
	github   gitHubClient
	notifier notifier
	interval time.Duration
	log      *slog.Logger
}

func NewScanner(
	repos gitHubRepoRepository,
	subs subscriptionRepository,
	gh gitHubClient,
	notifier notifier,
	interval time.Duration,
	log *slog.Logger,
) *Scanner {
	return &Scanner{
		repos:    repos,
		subs:     subs,
		github:   gh,
		notifier: notifier,
		interval: interval,
		log:      log.With("component", "scanner"),
	}
}

func (s *Scanner) Start(ctx context.Context) {
	s.log.Info("scanner started", "interval", s.interval)

	s.scan(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("scanner stopped")
			return
		case <-ticker.C:
			s.scan(ctx)
		}
	}
}

func (s *Scanner) scan(ctx context.Context) {
	s.log.Debug("scanning repositories for new releases")

	start := time.Now()
	defer func() {
		metrics.ScannerDuration.Observe(time.Since(start).Seconds())
		metrics.ScannerRunsTotal.Inc()
	}()

	repos, err := s.repos.GetAllWithSubscriptions(ctx)
	if err != nil {
		s.log.Error("failed to get repositories", "error", err)
		return
	}

	s.log.Debug("found repos to scan", "count", len(repos))

	for _, repo := range repos {
		if err := s.checkRepo(ctx, repo); err != nil {
			s.log.Error("failed to check repository", "repo", repo.Name, "error", err)
		}
	}
}

func (s *Scanner) checkRepo(ctx context.Context, repo *domain.Repository) error {
	owner, name, err := domain.ParseRepo(repo.Name)
	if err != nil {
		return err
	}

	release, err := s.getRelease(ctx, repo.Name, owner, name)
	if err != nil {
		return err
	}

	if release == nil {
		return nil
	}

	if repo.LastSeenTag != nil && *repo.LastSeenTag == release.TagName {
		return nil
	}

	s.log.Info("new release detected", "repo", repo.Name, "tag", release.TagName)

	return s.notify(ctx, repo, release)
}

func (s *Scanner) getRelease(ctx context.Context, repoName, owner, name string) (*domain.Release, error) {
	release, err := s.github.GetLatestRelease(ctx, owner, name)
	if err != nil {
		return nil, s.handleReleaseError(err, repoName)
	}

	return release, nil
}

func (s *Scanner) handleReleaseError(err error, repoName string) error {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		metrics.GitHubAPIErrorsTotal.WithLabelValues("unauthorized").Inc()
		s.log.Warn("GitHub token is invalid or missing, skipping scan", "repo", repoName)
		return nil

	case errors.Is(err, domain.ErrRateLimited):
		metrics.GitHubAPIErrorsTotal.WithLabelValues("rate_limited").Inc()
		s.log.Warn("rate limited by GitHub, skipping scan", "repo", repoName)
		return nil

	case errors.Is(err, domain.ErrNoRelease):
		return nil

	default:
		metrics.GitHubAPIErrorsTotal.WithLabelValues("other").Inc()
		return fmt.Errorf("get latest release: %w", err)
	}
}

func (s *Scanner) notify(ctx context.Context, repo *domain.Repository, release *domain.Release) error {
	subs, err := s.subs.GetConfirmedByRepoID(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("get subscribers: %w", err)
	}

	if len(subs) == 0 {
		if err := s.repos.UpdateLastSeenTag(ctx, repo.Name, release.TagName); err != nil {
			return fmt.Errorf("update last seen tag: %w", err)
		}

		return nil
	}

	if err := s.notifier.Notify(ctx, subs, repo, release); err != nil {
		return fmt.Errorf("send notifications: %w", err)
	}

	if err := s.repos.UpdateLastSeenTag(ctx, repo.Name, release.TagName); err != nil {
		return fmt.Errorf("update last seen tag: %w", err)
	}

	metrics.NotificationsSentTotal.Add(float64(len(subs)))
	s.log.Info("notifications sent", "repo", repo.Name, "tag", release.TagName, "count", len(subs))

	return nil
}
