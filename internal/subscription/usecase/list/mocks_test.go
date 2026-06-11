package list_test

import (
	"context"

	"github-release-notifier/internal/subscription/domain"

	"github.com/stretchr/testify/mock"
)

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) GetByEmail(ctx context.Context, email string) ([]*domain.SubscriptionView, error) {
	args := m.Called(ctx, email)
	v, _ := args.Get(0).([]*domain.SubscriptionView)
	return v, args.Error(1)
}
