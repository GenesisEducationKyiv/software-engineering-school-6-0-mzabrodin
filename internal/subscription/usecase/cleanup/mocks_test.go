package cleanup_test

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/subscription/domain"
	"github-release-notifier/internal/subscription/usecase/cleanup"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) DeleteExpiredPending(
	ctx context.Context,
	cutoff time.Time,
) ([]domain.ExpiredSubscription, error) {
	args := m.Called(ctx, cutoff)
	v, _ := args.Get(0).([]domain.ExpiredSubscription)

	return v, args.Error(1)
}

type mockPublisher struct{ mock.Mock }

func (m *mockPublisher) SubscriptionExpired(ctx context.Context, ev events.SubscriptionExpired) error {
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

func (m mocks) useCase(maxAge time.Duration) *cleanup.UseCase {
	return cleanup.New(m.subs, m.tx, m.pub, maxAge, testLogger)
}

func (m mocks) assertExpectations(t mock.TestingT) {
	m.subs.AssertExpectations(t)
	m.tx.AssertExpectations(t)
	m.pub.AssertExpectations(t)
}
