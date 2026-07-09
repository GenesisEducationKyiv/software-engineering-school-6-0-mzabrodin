package confirm_test

import (
	"context"
	"io"
	"log/slog"

	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/subscription/domain"
	"github-release-notifier/internal/subscription/usecase/confirm"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) Confirm(ctx context.Context, email, repo string) (domain.ConfirmResult, error) {
	args := m.Called(ctx, email, repo)
	res, _ := args.Get(0).(domain.ConfirmResult)

	return res, args.Error(1)
}

type mockTokenVerifier struct{ mock.Mock }

func (m *mockTokenVerifier) Verify(token string) (email, repo string, err error) {
	args := m.Called(token)

	return args.String(0), args.String(1), args.Error(2)
}

type mockPublisher struct{ mock.Mock }

func (m *mockPublisher) SubscriptionConfirmed(ctx context.Context, ev events.SubscriptionConfirmed) error {
	return m.Called(ctx, ev).Error(0)
}

func (m *mockPublisher) Notify() { m.Called() }

type mockTransactor struct{ mock.Mock }

func (m *mockTransactor) Within(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := m.Called(ctx).Error(0); err != nil {
		return err
	}

	return fn(ctx)
}

type mocks struct {
	subs   *mockSubRepository
	tokens *mockTokenVerifier
	tx     *mockTransactor
	pub    *mockPublisher
}

func newMocks() mocks {
	return mocks{
		subs:   &mockSubRepository{},
		tokens: &mockTokenVerifier{},
		tx:     &mockTransactor{},
		pub:    &mockPublisher{},
	}
}

func (m mocks) useCase() *confirm.UseCase {
	return confirm.New(m.subs, m.tokens, m.tx, m.pub, testLogger)
}

func (m mocks) assertExpectations(t mock.TestingT) {
	m.subs.AssertExpectations(t)
	m.tokens.AssertExpectations(t)
	m.tx.AssertExpectations(t)
	m.pub.AssertExpectations(t)
}
