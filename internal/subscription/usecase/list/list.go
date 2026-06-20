package list

import (
	"context"

	"github-release-notifier/internal/subscription/domain"
)

type subRepository interface {
	GetByEmail(ctx context.Context, email string) ([]domain.SubscriptionView, error)
}

type Input struct {
	Email string
}

type Output struct {
	Views []domain.SubscriptionView
}

type UseCase struct {
	subs subRepository
}

func New(subs subRepository) *UseCase {
	return &UseCase{subs: subs}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	views, err := uc.subs.GetByEmail(ctx, in.Email)
	if err != nil {
		return Output{}, err
	}

	return Output{Views: views}, nil
}
