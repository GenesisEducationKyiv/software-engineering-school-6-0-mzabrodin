package scan

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/shared/entity"
)

type repositories interface {
	GetAllWithSubscriptions(ctx context.Context) ([]*entity.Repository, error)
	UpdateLastSeenTag(ctx context.Context, name, tag string) error
}

type subscriptions interface {
	GetConfirmedByRepoID(ctx context.Context, repoID uuid.UUID) ([]*entity.Subscription, error)
}

type releaseFetcher interface {
	FetchLatestReleases(ctx context.Context, repos []string) ([]entity.ObservedRelease, error)
}

type notifier interface {
	Notify(ctx context.Context, subs []*entity.Subscription, repo *entity.Repository, release *entity.Release) error
}

type Input struct{}

type Output struct{}

type UseCase struct {
	repos    repositories
	subs     subscriptions
	fetcher  releaseFetcher
	notifier notifier
	log      *slog.Logger
}

func New(repos repositories, subs subscriptions, fetcher releaseFetcher, n notifier, log *slog.Logger) *UseCase {
	return &UseCase{repos: repos, subs: subs, fetcher: fetcher, notifier: n, log: log.With("component", "scan")}
}

func (uc *UseCase) Execute(ctx context.Context, _ Input) (Output, error) {
	start := time.Now()
	defer metrics.RecordScanRun(start)

	repos, err := uc.repos.GetAllWithSubscriptions(ctx)
	if err != nil {
		metrics.ScannerErrorsTotal.WithLabelValues("list_repos").Inc()
		return Output{}, fmt.Errorf("list repositories: %w", err)
	}

	if len(repos) == 0 {
		return Output{}, nil
	}

	names := make([]string, len(repos))
	byName := make(map[string]*entity.Repository, len(repos))
	for i, repo := range repos {
		names[i] = repo.Name
		byName[repo.Name] = repo
	}

	observed, err := uc.fetcher.FetchLatestReleases(ctx, names)
	if err != nil {
		metrics.ScannerErrorsTotal.WithLabelValues("fetch_releases").Inc()
		return Output{}, fmt.Errorf("fetch latest releases: %w", err)
	}

	for _, rel := range observed {
		repo := byName[rel.Repo]
		if repo == nil {
			continue
		}

		if err := uc.process(ctx, repo, rel.Release); err != nil {
			uc.log.ErrorContext(ctx, "failed to process release", "repo", repo.Name, "error", err)
			metrics.ScannerErrorsTotal.WithLabelValues("process_release").Inc()
		}
	}

	return Output{}, nil
}

func (uc *UseCase) process(ctx context.Context, repo *entity.Repository, release *entity.Release) error {
	tag := release.TagName

	if repo.LastSeenTag == nil {
		uc.log.InfoContext(ctx, "seeding last seen tag", "repo", repo.Name, "tag", tag)

		if err := uc.repos.UpdateLastSeenTag(ctx, repo.Name, tag); err != nil {
			return fmt.Errorf("seed last seen tag: %w", err)
		}

		return nil
	}

	if *repo.LastSeenTag == tag {
		return nil
	}

	uc.log.InfoContext(ctx, "new release detected", "repo", repo.Name, "tag", tag)

	return uc.notify(ctx, repo, release)
}

func (uc *UseCase) notify(ctx context.Context, repo *entity.Repository, release *entity.Release) error {
	subs, err := uc.subs.GetConfirmedByRepoID(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("get subscribers: %w", err)
	}

	if len(subs) == 0 {
		if err := uc.repos.UpdateLastSeenTag(ctx, repo.Name, release.TagName); err != nil {
			return fmt.Errorf("update last seen tag: %w", err)
		}

		return nil
	}

	if err := uc.notifier.Notify(ctx, subs, repo, release); err != nil {
		return fmt.Errorf("send notifications: %w", err)
	}

	if err := uc.repos.UpdateLastSeenTag(ctx, repo.Name, release.TagName); err != nil {
		return fmt.Errorf("update last seen tag: %w", err)
	}

	metrics.NotificationsSentTotal.Add(float64(len(subs)))
	uc.log.InfoContext(ctx, "notifications sent", "repo", repo.Name, "tag", release.TagName, "count", len(subs))

	return nil
}
