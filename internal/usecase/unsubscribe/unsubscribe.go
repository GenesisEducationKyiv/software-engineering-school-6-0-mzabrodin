package unsubscribe

import (
	"context"
	"log/slog"
)

type subRepository interface {
	Delete(ctx context.Context, token string) error
}

type Input struct {
	Token string
}

type Output struct{}

type UseCase struct {
	subs subRepository
	log  *slog.Logger
}

func New(subs subRepository, log *slog.Logger) *UseCase {
	return &UseCase{subs: subs, log: log.With("component", "unsubscribe")}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	if err := uc.subs.Delete(ctx, in.Token); err != nil {
		return Output{}, err
	}

	uc.log.InfoContext(ctx, "subscription deleted")

	return Output{}, nil
}
