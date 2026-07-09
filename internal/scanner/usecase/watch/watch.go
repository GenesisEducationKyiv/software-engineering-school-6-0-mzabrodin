package watch

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/scanner/domain"
	shareddomain "github-release-notifier/internal/shared/domain"
	"github-release-notifier/internal/shared/events"
)

type repository interface {
	ListWatched(ctx context.Context) ([]domain.WatchedRepo, error)
	AdvanceTag(ctx context.Context, repoName, tag string) error
}

type scanner interface {
	Scan(ctx context.Context, repos []string) ([]domain.ObservedRelease, error)
}

type publisher interface {
	ReleaseDetected(ctx context.Context, ev events.ReleaseDetected) error
}

type UseCase struct {
	repos     repository
	scanner   scanner
	publisher publisher
	log       *slog.Logger
}

func New(repos repository, sc scanner, pub publisher, log *slog.Logger) *UseCase {
	return &UseCase{repos: repos, scanner: sc, publisher: pub, log: log.With("component", "watch")}
}

func (uc *UseCase) Run(ctx context.Context) error {
	start := time.Now()
	defer metrics.RecordScanRun(start)

	watched, err := uc.repos.ListWatched(ctx)
	if err != nil {
		metrics.ScannerErrorsTotal.WithLabelValues("list_repos").Inc()
		return fmt.Errorf("list watched repos: %w", err)
	}

	if len(watched) == 0 {
		uc.log.DebugContext(ctx, "no watched repos, skipping scan")
		return nil
	}

	names := make([]string, len(watched))
	byName := make(map[string]domain.WatchedRepo, len(watched))
	for i, repo := range watched {
		names[i] = repo.RepoName
		byName[repo.RepoName] = repo
	}

	uc.log.InfoContext(ctx, "scan started", "repos", len(names))

	observed, err := uc.scanner.Scan(ctx, names)
	if err != nil {
		metrics.ScannerErrorsTotal.WithLabelValues("scan").Inc()
		return fmt.Errorf("scan repos: %w", err)
	}

	var seeded, detected int
	for _, rel := range observed {
		repo, ok := byName[rel.Repo]
		if !ok {
			continue
		}

		switch uc.process(ctx, repo, rel.Release) {
		case outcomeSeeded:
			seeded++
		case outcomeDetected:
			detected++
		case outcomeUnchanged:
		}
	}

	uc.log.InfoContext(ctx, "scan completed",
		"repos", len(names), "fetched", len(observed),
		"detected", detected, "seeded", seeded, "duration", time.Since(start).String())

	return nil
}

type outcome int

const (
	outcomeUnchanged outcome = iota
	outcomeSeeded
	outcomeDetected
)

func (uc *UseCase) process(ctx context.Context, repo domain.WatchedRepo, release *shareddomain.Release) outcome {
	tag := release.TagName

	if repo.LastSeenTag == nil {
		uc.log.InfoContext(ctx, "seeding last seen tag", "repo", repo.RepoName, "tag", tag)

		if err := uc.repos.AdvanceTag(ctx, repo.RepoName, tag); err != nil {
			metrics.ScannerErrorsTotal.WithLabelValues("seed_tag").Inc()
			uc.log.ErrorContext(ctx, "failed to seed last seen tag", "repo", repo.RepoName, "error", err)
		}

		return outcomeSeeded
	}

	if *repo.LastSeenTag == tag {
		uc.log.DebugContext(ctx, "no new release", "repo", repo.RepoName, "tag", tag)
		return outcomeUnchanged
	}

	uc.log.InfoContext(ctx, "new release detected",
		"repo", repo.RepoName, "tag", tag, "previous_tag", *repo.LastSeenTag)

	if err := uc.publisher.ReleaseDetected(ctx, events.ReleaseDetected{
		SagaID:     uuid.NewString(),
		RepoName:   repo.RepoName,
		Tag:        tag,
		ReleaseURL: release.HTMLURL,
	}); err != nil {
		metrics.ScannerErrorsTotal.WithLabelValues("publish_detected").Inc()
		uc.log.ErrorContext(ctx, "failed to publish release detected", "repo", repo.RepoName, "tag", tag, "error", err)
	}

	return outcomeDetected
}
