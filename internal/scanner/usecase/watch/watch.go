package watch

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/scanner/domain"
	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/events"
)

type repository interface {
	ListWatched(ctx context.Context) ([]domain.WatchedRepo, error)
	AdvanceTag(ctx context.Context, repoName, tag string) error
}

type scanner interface {
	Scan(ctx context.Context, repos []string) ([]entity.ObservedRelease, error)
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
		return nil
	}

	names := make([]string, len(watched))
	byName := make(map[string]domain.WatchedRepo, len(watched))
	for i, repo := range watched {
		names[i] = repo.RepoName
		byName[repo.RepoName] = repo
	}

	observed, err := uc.scanner.Scan(ctx, names)
	if err != nil {
		metrics.ScannerErrorsTotal.WithLabelValues("scan").Inc()
		return fmt.Errorf("scan repos: %w", err)
	}

	for _, rel := range observed {
		repo, ok := byName[rel.Repo]
		if !ok {
			continue
		}

		uc.process(ctx, repo, rel.Release)
	}

	return nil
}

func (uc *UseCase) process(ctx context.Context, repo domain.WatchedRepo, release *entity.Release) {
	tag := release.TagName

	if repo.LastSeenTag == nil {
		uc.log.InfoContext(ctx, "seeding last seen tag", "repo", repo.RepoName, "tag", tag)

		if err := uc.repos.AdvanceTag(ctx, repo.RepoName, tag); err != nil {
			metrics.ScannerErrorsTotal.WithLabelValues("seed_tag").Inc()
			uc.log.ErrorContext(ctx, "failed to seed last seen tag", "repo", repo.RepoName, "error", err)
		}

		return
	}

	if *repo.LastSeenTag == tag {
		return
	}

	uc.log.InfoContext(ctx, "new release detected", "repo", repo.RepoName, "tag", tag)

	// Publish only; last_seen_tag advances when releases.notified reports a
	// successful delivery. A failed publish is re-detected next pass (the tag is
	// unchanged), so direct publishing needs no outbox.
	if err := uc.publisher.ReleaseDetected(ctx, events.ReleaseDetected{
		SagaID:     uuid.NewString(),
		RepoName:   repo.RepoName,
		Tag:        tag,
		ReleaseURL: release.HTMLURL,
	}); err != nil {
		metrics.ScannerErrorsTotal.WithLabelValues("publish_detected").Inc()
		uc.log.ErrorContext(ctx, "failed to publish release detected", "repo", repo.RepoName, "tag", tag, "error", err)
	}
}
