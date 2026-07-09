package coordinator

import (
	"context"

	"github-release-notifier/internal/saga/domain"
)

type compensationClient interface {
	Compensate(ctx context.Context, sagaID, sagaType, email, repoName string) (bool, error)
}

type GRPCCompensator struct {
	repo   transitioner
	client compensationClient
}

func NewGRPCCompensator(repo transitioner, client compensationClient) *GRPCCompensator {
	return &GRPCCompensator{repo: repo, client: client}
}

func (g *GRPCCompensator) Compensate(ctx context.Context, saga domain.Saga) error {
	if _, err := g.repo.TransitionByID(ctx, saga.ID, domain.StateCompensated); err != nil {
		return err
	}

	if _, err := g.client.Compensate(ctx, saga.ID.String(), string(saga.Type), saga.Email, saga.RepoName); err != nil {
		return err
	}

	return nil
}
