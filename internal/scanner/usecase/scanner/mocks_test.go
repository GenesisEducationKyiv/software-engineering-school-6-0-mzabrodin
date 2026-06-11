package scanner

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/shared/entity"
)

type mockRepoRepository struct {
	mock.Mock
}

func (m *mockRepoRepository) GetAllWithSubscriptions(ctx context.Context) ([]*entity.Repository, error) {
	args := m.Called(ctx)
	v, _ := args.Get(0).([]*entity.Repository)
	return v, args.Error(1)
}

func (m *mockRepoRepository) UpdateLastSeenTag(ctx context.Context, name, tag string) error {
	return m.Called(ctx, name, tag).Error(0)
}

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) GetConfirmedByRepoID(ctx context.Context, id uuid.UUID) ([]*entity.Subscription, error) {
	args := m.Called(ctx, id)
	v, _ := args.Get(0).([]*entity.Subscription)
	return v, args.Error(1)
}

type mockGitHub struct{ mock.Mock }

func (m *mockGitHub) GetLatestRelease(ctx context.Context, owner, repo string) (*entity.Release, error) {
	args := m.Called(ctx, owner, repo)
	v, _ := args.Get(0).(*entity.Release)
	return v, args.Error(1)
}

type mockNotifier struct{ mock.Mock }

func (m *mockNotifier) Notify(
	ctx context.Context,
	subs []*entity.Subscription,
	repo *entity.Repository,
	release *entity.Release,
) error {
	return m.Called(ctx, subs, repo, release).Error(0)
}
