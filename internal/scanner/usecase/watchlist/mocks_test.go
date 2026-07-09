package watchlist

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type mockRepository struct{ mock.Mock }

func (m *mockRepository) IncrementSubscriber(ctx context.Context, repoName string) error {
	return m.Called(ctx, repoName).Error(0)
}

func (m *mockRepository) DecrementSubscriber(ctx context.Context, repoName string) error {
	return m.Called(ctx, repoName).Error(0)
}
