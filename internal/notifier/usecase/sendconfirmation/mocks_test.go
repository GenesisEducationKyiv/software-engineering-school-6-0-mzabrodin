package sendconfirmation

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/notifier/domain"
	"github-release-notifier/internal/shared/events"
)

type mockSender struct{ mock.Mock }

func (m *mockSender) DeliverConfirmation(ctx context.Context, to, repo, confirmURL string) error {
	return m.Called(ctx, to, repo, confirmURL).Error(0)
}

type mockFailedStore struct{ mock.Mock }

func (m *mockFailedStore) Add(ctx context.Context, fc *domain.FailedConfirmation) error {
	return m.Called(ctx, fc).Error(0)
}

type mockPublisher struct{ mock.Mock }

func (m *mockPublisher) ConfirmationSent(ctx context.Context, ev events.NotificationConfirmationSent) error {
	return m.Called(ctx, ev).Error(0)
}

func (m *mockPublisher) ConfirmationFailed(ctx context.Context, ev events.NotificationConfirmationFailed) error {
	return m.Called(ctx, ev).Error(0)
}
