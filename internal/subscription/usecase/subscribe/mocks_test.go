package subscribe_test

import (
	"context"
	"io"
	"log/slog"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/subscription/domain"
	"github-release-notifier/internal/subscription/usecase/subscribe"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockRepoRepository struct{ mock.Mock }

func (m *mockRepoRepository) GetOrCreate(ctx context.Context, name string) (domain.Repository, error) {
	args := m.Called(ctx, name)
	v, _ := args.Get(0).(domain.Repository)

	return v, args.Error(1)
}

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) Create(ctx context.Context, sub domain.Subscription) error {
	return m.Called(ctx, sub).Error(0)
}

func (m *mockSubRepository) FindByEmailAndRepo(
	ctx context.Context,
	email string,
	repoID uuid.UUID,
) (domain.Subscription, error) {
	args := m.Called(ctx, email, repoID)
	v, _ := args.Get(0).(domain.Subscription)

	return v, args.Error(1)
}

type mockGitHub struct{ mock.Mock }

func (m *mockGitHub) RepoExists(ctx context.Context, owner, repo string) (bool, error) {
	args := m.Called(ctx, owner, repo)

	return args.Bool(0), args.Error(1)
}

type mockTokenIssuer struct{ mock.Mock }

func (m *mockTokenIssuer) Issue(email, repo string) (string, error) {
	args := m.Called(email, repo)

	return args.String(0), args.Error(1)
}

type mockPublisher struct{ mock.Mock }

func (m *mockPublisher) SubscriptionPending(ctx context.Context, ev events.SubscriptionPending) error {
	return m.Called(ctx, ev).Error(0)
}

func (m *mockPublisher) Notify() { m.Called() }

type mockURLBuilder struct{ mock.Mock }

func (m *mockURLBuilder) ConfirmURL(token string) string {
	return m.Called(token).String(0)
}

type mockTransactor struct{ mock.Mock }

func (m *mockTransactor) Within(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := m.Called(ctx).Error(0); err != nil {
		return err
	}

	return fn(ctx)
}

type mocks struct {
	repos  *mockRepoRepository
	subs   *mockSubRepository
	gh     *mockGitHub
	tokens *mockTokenIssuer
	urls   *mockURLBuilder
	tx     *mockTransactor
	pub    *mockPublisher
}

func newMocks() mocks {
	return mocks{
		repos:  &mockRepoRepository{},
		subs:   &mockSubRepository{},
		gh:     &mockGitHub{},
		tokens: &mockTokenIssuer{},
		urls:   &mockURLBuilder{},
		tx:     &mockTransactor{},
		pub:    &mockPublisher{},
	}
}

func (m mocks) useCase() *subscribe.UseCase {
	return subscribe.New(m.repos, m.subs, m.gh, m.tokens, m.urls, m.tx, m.pub, testLogger)
}

func (m mocks) expectRepoResolved(repoID uuid.UUID) {
	m.gh.On("RepoExists", mock.Anything, "owner", "repo").Return(true, nil)
	m.repos.On("GetOrCreate", mock.Anything, "owner/repo").
		Return(domain.Repository{ID: repoID, Name: "owner/repo"}, nil)
}

func (m mocks) assertExpectations(t mock.TestingT) {
	m.repos.AssertExpectations(t)
	m.subs.AssertExpectations(t)
	m.gh.AssertExpectations(t)
	m.tokens.AssertExpectations(t)
	m.urls.AssertExpectations(t)
	m.tx.AssertExpectations(t)
	m.pub.AssertExpectations(t)
}
