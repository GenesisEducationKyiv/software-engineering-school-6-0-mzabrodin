package watchlist

import (
	"context"
	"fmt"
	"log/slog"

	"github-release-notifier/internal/shared/events"
)

type repository interface {
	IncrementSubscriber(ctx context.Context, repoName string) error
	DecrementSubscriber(ctx context.Context, repoName string) error
}

type Projector struct {
	repos repository
	log   *slog.Logger
}

func New(repos repository, log *slog.Logger) *Projector {
	return &Projector{repos: repos, log: log.With("component", "watchlist")}
}

func (p *Projector) Confirmed(ctx context.Context, ev events.SubscriptionConfirmed) error {
	if err := p.repos.IncrementSubscriber(ctx, ev.RepoName); err != nil {
		return fmt.Errorf("increment subscriber: %w", err)
	}

	p.log.InfoContext(ctx, "subscriber added to watchlist", "repo", ev.RepoName)

	return nil
}

func (p *Projector) Removed(ctx context.Context, ev events.SubscriptionRemoved) error {
	if err := p.repos.DecrementSubscriber(ctx, ev.RepoName); err != nil {
		return fmt.Errorf("decrement subscriber: %w", err)
	}

	p.log.InfoContext(ctx, "subscriber removed from watchlist", "repo", ev.RepoName)

	return nil
}
