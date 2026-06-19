package readmodel

import (
	"context"
	"fmt"

	"github-release-notifier/internal/shared/events"
)

type store interface {
	Upsert(ctx context.Context, email, repoName, unsubToken string) error
	Delete(ctx context.Context, email, repoName string) error
}

type Projector struct {
	store store
}

func New(store store) *Projector {
	return &Projector{store: store}
}

func (p *Projector) Confirmed(ctx context.Context, ev events.SubscriptionConfirmed) error {
	if err := p.store.Upsert(ctx, ev.Email, ev.RepoName, ev.UnsubToken); err != nil {
		return fmt.Errorf("project subscription confirmed: %w", err)
	}

	return nil
}

func (p *Projector) Removed(ctx context.Context, ev events.SubscriptionRemoved) error {
	if err := p.store.Delete(ctx, ev.Email, ev.RepoName); err != nil {
		return fmt.Errorf("project subscription removed: %w", err)
	}

	return nil
}
