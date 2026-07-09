package coordinator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github-release-notifier/internal/saga/domain"
)

type repository interface {
	Start(ctx context.Context, s domain.Saga) error
	Get(ctx context.Context, id uuid.UUID) (domain.Saga, bool, error)
	TransitionByID(ctx context.Context, id uuid.UUID, to domain.State) (bool, error)
	TransitionByEmailRepo(ctx context.Context, t domain.SagaType, email, repoName string, to domain.State) (bool, error)
}

type compensator interface {
	Compensate(ctx context.Context, saga domain.Saga) error
}

type Coordinator struct {
	repo repository
	comp compensator
	log  *slog.Logger
}

func New(repo repository, comp compensator, log *slog.Logger) *Coordinator {
	return &Coordinator{repo: repo, comp: comp, log: log.With("component", "saga-coordinator")}
}

func (c *Coordinator) OnPending(ctx context.Context, sagaID, email, repoName string) error {
	id, err := uuid.Parse(sagaID)
	if err != nil {
		return fmt.Errorf("parse saga id: %w", err)
	}

	return c.repo.Start(ctx, domain.Saga{
		ID:       id,
		Type:     domain.SagaTypeSubscribe,
		Email:    email,
		RepoName: repoName,
	})
}

func (c *Coordinator) OnConfirmationSent(ctx context.Context, sagaID string) error {
	id, err := uuid.Parse(sagaID)
	if err != nil {
		return fmt.Errorf("parse saga id: %w", err)
	}

	if _, err := c.repo.TransitionByID(ctx, id, domain.StateConfirmationSent); err != nil {
		return err
	}

	return nil
}

func (c *Coordinator) OnConfirmed(ctx context.Context, email, repoName string) error {
	if _, err := c.repo.TransitionByEmailRepo(
		ctx, domain.SagaTypeSubscribe, email, repoName, domain.StateCompleted,
	); err != nil {
		return err
	}

	return nil
}

func (c *Coordinator) OnExpired(ctx context.Context, email, repoName string) error {
	if _, err := c.repo.TransitionByEmailRepo(
		ctx, domain.SagaTypeSubscribe, email, repoName, domain.StateExpired,
	); err != nil {
		return err
	}

	return nil
}

func (c *Coordinator) OnConfirmationDead(ctx context.Context, sagaID string) error {
	id, err := uuid.Parse(sagaID)
	if err != nil {
		return fmt.Errorf("parse saga id: %w", err)
	}

	saga, ok, err := c.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	if !ok {
		c.log.WarnContext(ctx, "confirmation dead for unknown saga", "saga_id", sagaID)

		return nil
	}

	switch saga.State {
	case domain.StateCompleted, domain.StateExpired, domain.StateCompensated:
		return nil
	case domain.StatePending, domain.StateConfirmationSent:
		if err := c.comp.Compensate(ctx, saga); err != nil {
			return err
		}

		c.log.InfoContext(ctx, "saga compensation triggered",
			"saga_id", sagaID, "email", saga.Email, "repo", saga.RepoName)
	}

	return nil
}
