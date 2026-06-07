package subscribe_test

import (
	"context"
	"io"
	"log/slog"

	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/usecase/subscribe"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockRepoRepository struct{ mock.Mock }

func (m *mockRepoRepository) Create(ctx context.Context, repo *entity.Repository) error {
	return m.Called(ctx, repo).Error(0)
}

func (m *mockRepoRepository) GetByName(ctx context.Context, name string) (*entity.Repository, error) {
	args := m.Called(ctx, name)
	v, _ := args.Get(0).(*entity.Repository)
	return v, args.Error(1)
}

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) Create(ctx context.Context, sub *entity.Subscription) error {
	return m.Called(ctx, sub).Error(0)
}

type mockGitHub struct{ mock.Mock }

func (m *mockGitHub) RepoExists(ctx context.Context, owner, repo string) (bool, error) {
	args := m.Called(ctx, owner, repo)
	return args.Bool(0), args.Error(1)
}

type mockMailer struct{ mock.Mock }

func (m *mockMailer) SendConfirmation(_ context.Context, to, repo, confirmURL string) {
	m.Called(to, repo, confirmURL)
}

type mockURLBuilder struct{}

func (m *mockURLBuilder) ConfirmURL(token string) string {
	return "http://localhost:8080/api/confirm/" + token
}

func newUseCase(
	repos *mockRepoRepository,
	subs *mockSubRepository,
	gh *mockGitHub,
	mailer *mockMailer,
) *subscribe.UseCase {
	return subscribe.New(repos, subs, gh, mailer, &mockURLBuilder{}, testLogger)
}
