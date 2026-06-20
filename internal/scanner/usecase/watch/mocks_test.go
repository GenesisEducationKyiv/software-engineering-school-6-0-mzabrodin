package watch

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/scanner/domain"
	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/events"
)

type mockRepository struct{ mock.Mock }

func (m *mockRepository) ListWatched(ctx context.Context) ([]domain.WatchedRepo, error) {
	args := m.Called(ctx)

	repos, _ := args.Get(0).([]domain.WatchedRepo)

	return repos, args.Error(1)
}

func (m *mockRepository) AdvanceTag(ctx context.Context, repoName, tag string) error {
	return m.Called(ctx, repoName, tag).Error(0)
}

type mockScanner struct{ mock.Mock }

func (m *mockScanner) Scan(ctx context.Context, repos []string) ([]entity.ObservedRelease, error) {
	args := m.Called(ctx, repos)

	observed, _ := args.Get(0).([]entity.ObservedRelease)

	return observed, args.Error(1)
}

type mockPublisher struct{ mock.Mock }

func (m *mockPublisher) ReleaseDetected(ctx context.Context, ev events.ReleaseDetected) error {
	return m.Called(ctx, ev).Error(0)
}
