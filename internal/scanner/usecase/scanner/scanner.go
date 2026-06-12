package scanner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/github"
)

type notifier interface {
	Notify(ctx context.Context, subs []*entity.Subscription, repo *entity.Repository, release *entity.Release) error
}

type gitHubClient interface {
	GetLatestRelease(ctx context.Context, owner, repo string) (*entity.Release, error)
}

type gitHubRepoRepository interface {
	GetAllWithSubscriptions(ctx context.Context) ([]*entity.Repository, error)
	UpdateLastSeenTag(ctx context.Context, name string, tag string) error
}

type subscriptionRepository interface {
	GetConfirmedByRepoID(ctx context.Context, repoID uuid.UUID) ([]*entity.Subscription, error)
}

type Scanner struct {
	repos    gitHubRepoRepository
	subs     subscriptionRepository
	github   gitHubClient
	notifier notifier
	workers  int
	log      *slog.Logger
}

func NewScanner(
	repos gitHubRepoRepository,
	subs subscriptionRepository,
	gh gitHubClient,
	notifier notifier,
	workers int,
	log *slog.Logger,
) *Scanner {
	return &Scanner{
		repos:    repos,
		subs:     subs,
		github:   gh,
		notifier: notifier,
		workers:  workers,
		log:      log.With("component", "scanner"),
	}
}

func (s *Scanner) Run(ctx context.Context) error {
	s.log.InfoContext(ctx, "scanning repositories for new releases")

	start := time.Now()
	defer metrics.RecordScanRun(start)

	repos, err := s.repos.GetAllWithSubscriptions(ctx)
	if err != nil {
		metrics.ScannerErrorsTotal.WithLabelValues("fetch_repos").Inc()
		return fmt.Errorf("get repositories with subscriptions: %w", err)
	}

	s.log.InfoContext(ctx, "found repos to scan", "count", len(repos))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(s.workers)

	for _, repo := range repos {
		if gctx.Err() != nil {
			break
		}

		g.Go(func() error {
			return s.scanRepo(gctx, repo)
		})
	}

	if err := g.Wait(); err != nil {
		s.log.WarnContext(ctx, "scan pass stopped early", "reason", err)
	}

	s.log.InfoContext(ctx, "scan completed", "repos", len(repos), "duration_ms", time.Since(start).Milliseconds())

	return nil
}

func (s *Scanner) scanRepo(ctx context.Context, repo *entity.Repository) error {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	err := s.checkRepo(ctx, repo)
	if errors.Is(err, github.ErrRateLimited) || errors.Is(err, github.ErrUnauthorized) {
		return err
	}

	// Per-repo failures are isolated: log, record and keep scanning the rest.
	if err != nil && !errors.Is(err, context.Canceled) {
		s.log.ErrorContext(ctx, "failed to check repository", "repo", repo.Name, "error", err)
		metrics.ScannerErrorsTotal.WithLabelValues("check_repo").Inc()
	}

	return nil //nolint:nilerr // per-repo errors are intentionally isolated above
}

func (s *Scanner) checkRepo(ctx context.Context, repo *entity.Repository) error {
	owner, name, err := github.ParseRepo(repo.Name)
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

	s.log.InfoContext(ctx, "new release detected", "repo", repo.Name, "tag", release.TagName)

	return s.notify(ctx, repo, release)
}

func (s *Scanner) getRelease(ctx context.Context, repoName, owner, name string) (*entity.Release, error) {
	release, err := s.github.GetLatestRelease(ctx, owner, name)
	if err != nil {
		return nil, s.handleReleaseError(ctx, err, repoName)
	}

	return release, nil
}

func (s *Scanner) handleReleaseError(ctx context.Context, err error, repoName string) error {
	switch {
	case errors.Is(err, github.ErrUnauthorized):
		metrics.GitHubAPIErrorsTotal.WithLabelValues("unauthorized").Inc()
		s.log.WarnContext(ctx, "GitHub token is invalid or missing, stopping scan", "repo", repoName)
		return err

	case errors.Is(err, github.ErrRateLimited):
		metrics.GitHubAPIErrorsTotal.WithLabelValues("rate_limited").Inc()
		s.log.WarnContext(ctx, "rate limited by GitHub, stopping scan", "repo", repoName)
		return err

	case errors.Is(err, github.ErrNoRelease):
		return nil

	default:
		metrics.GitHubAPIErrorsTotal.WithLabelValues("other").Inc()
		return fmt.Errorf("get latest release: %w", err)
	}
}

func (s *Scanner) notify(ctx context.Context, repo *entity.Repository, release *entity.Release) error {
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
	s.log.InfoContext(ctx, "notifications sent", "repo", repo.Name, "tag", release.TagName, "count", len(subs))

	return nil
}
