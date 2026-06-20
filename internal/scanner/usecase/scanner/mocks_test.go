package scanner

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/shared/domain"
)

type mockGitHub struct{ mock.Mock }

func (m *mockGitHub) GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error) {
	args := m.Called(ctx, owner, repo)
	v, _ := args.Get(0).(*domain.Release)
	return v, args.Error(1)
}
