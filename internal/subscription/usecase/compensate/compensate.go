package compensate

import (
	"context"
	"log/slog"
)

type subRepository interface {
	DeletePendingByEmailAndRepo(ctx context.Context, email, repoName string) (bool, error)
}

type Input struct {
	Email    string
	RepoName string
}

type UseCase struct {
	subs subRepository
	log  *slog.Logger
}

func New(subs subRepository, log *slog.Logger) *UseCase {
	return &UseCase{subs: subs, log: log.With("component", "compensate")}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) (bool, error) {
	deleted, err := uc.subs.DeletePendingByEmailAndRepo(ctx, in.Email, in.RepoName)
	if err != nil {
		return false, err
	}

	if deleted {
		uc.log.InfoContext(ctx, "pending subscription rolled back", "email", in.Email, "repo", in.RepoName)
	}

	return deleted, nil
}
