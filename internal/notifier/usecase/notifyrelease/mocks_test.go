package notifyrelease

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/domain"
	"github-release-notifier/internal/shared/events"
)

type mockRecipients struct{ mock.Mock }

func (m *mockRecipients) RecipientsByRepo(ctx context.Context, repoName string) ([]domain.Recipient, error) {
	args := m.Called(ctx, repoName)
	v, _ := args.Get(0).([]domain.Recipient)

	return v, args.Error(1)
}

type mockDedupe struct{ mock.Mock }

func (m *mockDedupe) Exists(ctx context.Context, repoName, tag string) (bool, error) {
	args := m.Called(ctx, repoName, tag)

	return args.Bool(0), args.Error(1)
}

func (m *mockDedupe) Mark(ctx context.Context, repoName, tag string) error {
	return m.Called(ctx, repoName, tag).Error(0)
}

type mockFailedStore struct{ mock.Mock }

func (m *mockFailedStore) Add(ctx context.Context, fn *domain.FailedNotification) error {
	return m.Called(ctx, fn).Error(0)
}

type mockSender struct{ mock.Mock }

func (m *mockSender) SendReleaseNotifications(
	ctx context.Context,
	notifications []notifier.ReleaseNotification,
) notifier.BatchResult {
	args := m.Called(ctx, notifications)
	v, _ := args.Get(0).(notifier.BatchResult)

	return v
}

type mockURLs struct{ mock.Mock }

func (m *mockURLs) UnsubscribeURL(token string) string {
	return m.Called(token).String(0)
}

type mockPublisher struct{ mock.Mock }

func (m *mockPublisher) ReleaseSent(ctx context.Context, ev events.NotificationReleaseSent) error {
	return m.Called(ctx, ev).Error(0)
}

func (m *mockPublisher) ReleaseFailed(ctx context.Context, ev events.NotificationReleaseFailed) error {
	return m.Called(ctx, ev).Error(0)
}

func (m *mockPublisher) ReleaseNotified(ctx context.Context, ev events.ReleaseNotified) error {
	return m.Called(ctx, ev).Error(0)
}

func (m *mockPublisher) Notify() { m.Called() }

type mockTransactor struct{}

func (mockTransactor) Within(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
