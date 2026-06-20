package watchlist

import (
	"context"
	"fmt"

	"github-release-notifier/internal/shared/events"
)

type repository interface {
	IncrementSubscriber(ctx context.Context, repoName string) error
	DecrementSubscriber(ctx context.Context, repoName string) error
}

type Projector struct {
	repos repository
}

func New(repos repository) *Projector {
	return &Projector{repos: repos}
}

func (p *Projector) Confirmed(ctx context.Context, ev events.SubscriptionConfirmed) error {
	if err := p.repos.IncrementSubscriber(ctx, ev.RepoName); err != nil {
		return fmt.Errorf("increment subscriber: %w", err)
	}

	return nil
}

func (p *Projector) Removed(ctx context.Context, ev events.SubscriptionRemoved) error {
	if err := p.repos.DecrementSubscriber(ctx, ev.RepoName); err != nil {
		return fmt.Errorf("decrement subscriber: %w", err)
	}

	return nil
}
