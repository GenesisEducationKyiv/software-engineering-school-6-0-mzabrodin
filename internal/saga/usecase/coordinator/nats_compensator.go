package coordinator

import (
	"context"

	"github.com/google/uuid"

	"github-release-notifier/internal/saga/domain"
	"github-release-notifier/internal/shared/events"
)

type transitioner interface {
	TransitionByID(ctx context.Context, id uuid.UUID, to domain.State) (bool, error)
}

type publisher interface {
	Compensate(ctx context.Context, ev events.SagaCompensate) error
	Notify()
}

type transactor interface {
	Within(ctx context.Context, fn func(ctx context.Context) error) error
}

type NATSCompensator struct {
	repo transitioner
	pub  publisher
	tx   transactor
}

func NewNATSCompensator(repo transitioner, pub publisher, tx transactor) *NATSCompensator {
	return &NATSCompensator{repo: repo, pub: pub, tx: tx}
}

func (n *NATSCompensator) Compensate(ctx context.Context, saga domain.Saga) error {
	if err := n.tx.Within(ctx, func(txCtx context.Context) error {
		if _, err := n.repo.TransitionByID(txCtx, saga.ID, domain.StateCompensated); err != nil {
			return err
		}

		return n.pub.Compensate(txCtx, events.SagaCompensate{
			SagaID:   saga.ID.String(),
			SagaType: string(saga.Type),
			Email:    saga.Email,
			RepoName: saga.RepoName,
		})
	}); err != nil {
		return err
	}

	n.pub.Notify()

	return nil
}
