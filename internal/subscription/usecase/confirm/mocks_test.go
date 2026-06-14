package confirm_test

import (
	"context"
	"io"
	"log/slog"

	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/shared/entity"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) Confirm(ctx context.Context, token string) (*entity.Subscription, string, error) {
	args := m.Called(ctx, token)
	sub, _ := args.Get(0).(*entity.Subscription)

	return sub, args.String(1), args.Error(2)
}

type mockGitHub struct{ mock.Mock }

func (m *mockGitHub) GetLatestRelease(ctx context.Context, owner, repo string) (*entity.Release, error) {
	args := m.Called(ctx, owner, repo)
	rel, _ := args.Get(0).(*entity.Release)

	return rel, args.Error(1)
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
