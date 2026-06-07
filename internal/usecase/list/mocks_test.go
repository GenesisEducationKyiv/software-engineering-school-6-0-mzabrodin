package list_test

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/entity"
)

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) GetByEmail(ctx context.Context, email string) ([]*entity.SubscriptionView, error) {
	args := m.Called(ctx, email)
	v, _ := args.Get(0).([]*entity.SubscriptionView)
	return v, args.Error(1)
}
