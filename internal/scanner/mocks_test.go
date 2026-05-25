package scanner

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/domain"
)

type mockRepoRepository struct {
	mock.Mock
}

func (m *mockRepoRepository) GetAllWithSubscriptions(ctx context.Context) ([]*domain.Repository, error) {
	args := m.Called(ctx)
	v, _ := args.Get(0).([]*domain.Repository)
	return v, args.Error(1)
}

func (m *mockRepoRepository) UpdateLastSeenTag(ctx context.Context, name, tag string) error {
	return m.Called(ctx, name, tag).Error(0)
}

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) GetConfirmedByRepoID(ctx context.Context, id uuid.UUID) ([]*domain.Subscription, error) {
	args := m.Called(ctx, id)
	v, _ := args.Get(0).([]*domain.Subscription)
	return v, args.Error(1)
}

type mockGitHub struct{ mock.Mock }

func (m *mockGitHub) GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error) {
	args := m.Called(ctx, owner, repo)
	v, _ := args.Get(0).(*domain.Release)
	return v, args.Error(1)
}

type mockNotifier struct{ mock.Mock }

func (m *mockNotifier) Notify(
	ctx context.Context,
	subs []*domain.Subscription,
	repo *domain.Repository,
	release *domain.Release,
) error {
	return m.Called(ctx, subs, repo, release).Error(0)
}
