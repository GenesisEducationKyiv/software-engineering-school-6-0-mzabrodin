package list_test

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/subscription/domain"
	"github-release-notifier/internal/subscription/usecase/list"
)

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) GetByEmail(ctx context.Context, email string) ([]domain.SubscriptionView, error) {
	args := m.Called(ctx, email)
	v, _ := args.Get(0).([]domain.SubscriptionView)

	return v, args.Error(1)
}

type mocks struct {
	subs *mockSubRepository
}

func newMocks() mocks {
	return mocks{subs: &mockSubRepository{}}
}

func (m mocks) useCase() *list.UseCase {
	return list.New(m.subs)
}

func (m mocks) assertExpectations(t mock.TestingT) {
	m.subs.AssertExpectations(t)
}
