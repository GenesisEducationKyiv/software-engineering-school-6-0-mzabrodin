package scanner

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"golang.org/x/sync/errgroup"

	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/github"
)

type gitHubClient interface {
	GetLatestRelease(ctx context.Context, owner, repo string) (*entity.Release, error)
}

type Scanner struct {
	github  gitHubClient
	workers int
	log     *slog.Logger
}

func New(gh gitHubClient, workers int, log *slog.Logger) *Scanner {
	return &Scanner{github: gh, workers: workers, log: log.With("component", "scanner")}
}

func (s *Scanner) Scan(ctx context.Context, repos []string) ([]entity.ObservedRelease, error) {
	var (
		mu       sync.Mutex
		observed = make([]entity.ObservedRelease, 0, len(repos))
	)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(s.workers)

	for _, repo := range repos {
		if gctx.Err() != nil {
			break
		}

		g.Go(func() error {
			release, err := s.fetchRelease(gctx, repo)
			if err != nil {
				return err
			}

			if release == nil {
				return nil
			}

			mu.Lock()
			observed = append(observed, entity.ObservedRelease{Repo: repo, Release: release})
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		s.log.WarnContext(ctx, "fetch pass stopped early", "reason", err)
	}

	return observed, nil
}

func (s *Scanner) fetchRelease(ctx context.Context, repoName string) (*entity.Release, error) {
	select {
	case <-ctx.Done():
		return nil, nil
	default:
	}

	owner, name, err := github.ParseRepo(repoName)
	if err != nil {
		s.log.ErrorContext(ctx, "invalid repository name", "repo", repoName, "error", err)
		metrics.ScannerErrorsTotal.WithLabelValues("parse_repo").Inc()

		return nil, nil //nolint:nilerr // per-repo errors are intentionally isolated
	}

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
		s.log.WarnContext(ctx, "GitHub token is invalid or missing, stopping pass", "repo", repoName)
		return err

	case errors.Is(err, github.ErrRateLimited):
		metrics.GitHubAPIErrorsTotal.WithLabelValues("rate_limited").Inc()
		s.log.WarnContext(ctx, "rate limited by GitHub, stopping pass", "repo", repoName)
		return err

	case errors.Is(err, github.ErrNoRelease), errors.Is(err, context.Canceled):
		return nil

	default:
		metrics.GitHubAPIErrorsTotal.WithLabelValues("other").Inc()
		metrics.ScannerErrorsTotal.WithLabelValues("fetch_release").Inc()
		s.log.ErrorContext(ctx, "failed to fetch latest release", "repo", repoName, "error", err)

		return nil
	}
}
