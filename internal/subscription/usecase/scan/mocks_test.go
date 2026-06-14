package scan

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/shared/entity"
)

type mockRepos struct{ mock.Mock }

func (m *mockRepos) GetAllWithSubscriptions(ctx context.Context) ([]*entity.Repository, error) {
	args := m.Called(ctx)
	v, _ := args.Get(0).([]*entity.Repository)
	return v, args.Error(1)
}

func (m *mockRepos) UpdateLastSeenTag(ctx context.Context, name, tag string) error {
	return m.Called(ctx, name, tag).Error(0)
}

type mockSubs struct{ mock.Mock }

func (m *mockSubs) GetConfirmedByRepoID(ctx context.Context, id uuid.UUID) ([]*entity.Subscription, error) {
	args := m.Called(ctx, id)
	v, _ := args.Get(0).([]*entity.Subscription)
	return v, args.Error(1)
}

type mockFetcher struct{ mock.Mock }

func (m *mockFetcher) FetchLatestReleases(ctx context.Context, repos []string) ([]entity.ObservedRelease, error) {
	args := m.Called(ctx, repos)
	v, _ := args.Get(0).([]entity.ObservedRelease)
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
