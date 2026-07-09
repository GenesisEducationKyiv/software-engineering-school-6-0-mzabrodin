package coordinator

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/saga/domain"
	"github-release-notifier/internal/shared/events"
)

type mockRepo struct{ mock.Mock }

func (m *mockRepo) Start(ctx context.Context, s domain.Saga) error {
	return m.Called(ctx, s).Error(0)
}

func (m *mockRepo) Get(ctx context.Context, id uuid.UUID) (domain.Saga, bool, error) {
	args := m.Called(ctx, id)

	return args.Get(0).(domain.Saga), args.Bool(1), args.Error(2)
}

func (m *mockRepo) TransitionByID(ctx context.Context, id uuid.UUID, to domain.State) (bool, error) {
	args := m.Called(ctx, id, to)

	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) TransitionByEmailRepo(
	ctx context.Context,
	t domain.SagaType,
	email, repoName string,
	to domain.State,
) (bool, error) {
	args := m.Called(ctx, t, email, repoName, to)

	return args.Bool(0), args.Error(1)
}

type mockPublisher struct{ mock.Mock }

func (m *mockPublisher) Compensate(ctx context.Context, ev events.SagaCompensate) error {
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
