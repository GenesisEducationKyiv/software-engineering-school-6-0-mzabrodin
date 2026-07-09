package unsubscribe_test

import (
	"context"
	"io"
	"log/slog"

	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/subscription/domain"
	"github-release-notifier/internal/subscription/usecase/unsubscribe"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) Delete(ctx context.Context, token string) (domain.RemovedSubscription, error) {
	args := m.Called(ctx, token)
	res, _ := args.Get(0).(domain.RemovedSubscription)

	return res, args.Error(1)
}

type mockPublisher struct{ mock.Mock }

func (m *mockPublisher) SubscriptionRemoved(ctx context.Context, ev events.SubscriptionRemoved) error {
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
	subs *mockSubRepository
	tx   *mockTransactor
	pub  *mockPublisher
}

func newMocks() mocks {
	return mocks{
		subs: &mockSubRepository{},
		tx:   &mockTransactor{},
		pub:  &mockPublisher{},
	}
}

func (m mocks) useCase() *unsubscribe.UseCase {
	return unsubscribe.New(m.subs, m.tx, m.pub, testLogger)
}

func (m mocks) assertExpectations(t mock.TestingT) {
	m.subs.AssertExpectations(t)
	m.tx.AssertExpectations(t)
	m.pub.AssertExpectations(t)
}
