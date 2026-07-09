package readmodel

import (
	"context"
	"fmt"
	"log/slog"

	"github-release-notifier/internal/shared/events"
)

type store interface {
	Upsert(ctx context.Context, email, repoName, unsubToken string) error
	Delete(ctx context.Context, email, repoName string) error
}

type Projector struct {
	store store
	log   *slog.Logger
}

func New(store store, log *slog.Logger) *Projector {
	return &Projector{store: store, log: log.With("component", "read-model")}
}

func (p *Projector) Confirmed(ctx context.Context, ev events.SubscriptionConfirmed) error {
	if err := p.store.Upsert(ctx, ev.Email, ev.RepoName, ev.UnsubToken); err != nil {
		return fmt.Errorf("project subscription confirmed: %w", err)
	}

	p.log.InfoContext(ctx, "read model updated: subscriber confirmed", "email", ev.Email, "repo", ev.RepoName)

	return nil
}

func (p *Projector) Removed(ctx context.Context, ev events.SubscriptionRemoved) error {
	if err := p.store.Delete(ctx, ev.Email, ev.RepoName); err != nil {
		return fmt.Errorf("project subscription removed: %w", err)
	}

	p.log.InfoContext(ctx, "read model updated: subscriber removed", "email", ev.Email, "repo", ev.RepoName)

	return nil
}
