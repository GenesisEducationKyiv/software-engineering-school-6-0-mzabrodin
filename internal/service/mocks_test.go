package service_test

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/domain"
)

type mockRepoRepository struct{ mock.Mock }

func (m *mockRepoRepository) Create(ctx context.Context, repo *domain.Repository) error {
	return m.Called(ctx, repo).Error(0)
}

func (m *mockRepoRepository) GetByName(ctx context.Context, name string) (*domain.Repository, error) {
	args := m.Called(ctx, name)
	v, _ := args.Get(0).(*domain.Repository)
	return v, args.Error(1)
}

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) Create(ctx context.Context, sub *domain.Subscription) error {
	return m.Called(ctx, sub).Error(0)
}

func (m *mockSubRepository) GetByEmail(ctx context.Context, email string) ([]*domain.SubscriptionView, error) {
	args := m.Called(ctx, email)
	v, _ := args.Get(0).([]*domain.SubscriptionView)
	return v, args.Error(1)
}

func (m *mockSubRepository) Confirm(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}

func (m *mockSubRepository) Delete(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}

type mockGitHub struct{ mock.Mock }

func (m *mockGitHub) RepoExists(ctx context.Context, owner, repo string) (bool, error) {
	args := m.Called(ctx, owner, repo)
	return args.Bool(0), args.Error(1)
}

type mockMailer struct{ mock.Mock }

func (m *mockMailer) SendConfirmation(to, repo, confirmURL string) error {
	return m.Called(to, repo, confirmURL).Error(0)
}

func (m *mockMailer) Shutdown() {}

type mockAsyncMailer struct{ mock.Mock }

func (m *mockAsyncMailer) SendConfirmation(_ context.Context, to, repo, confirmURL string) error {
	return m.Called(to, repo, confirmURL).Error(0)
}

type mockURLBuilder struct{}

func (m *mockURLBuilder) ConfirmURL(token string) string {
	return "http://localhost:8080/api/confirm/" + token
}
