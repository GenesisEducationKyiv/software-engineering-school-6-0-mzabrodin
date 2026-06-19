package retry

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/domain"
	"github-release-notifier/internal/shared/events"
)

type mockFailedNotifications struct{ mock.Mock }

func (m *mockFailedNotifications) ListRetryable(
	ctx context.Context,
	maxRetries int,
) ([]domain.FailedNotification, error) {
	args := m.Called(ctx, maxRetries)
	v, _ := args.Get(0).([]domain.FailedNotification)

	return v, args.Error(1)
}

func (m *mockFailedNotifications) IncrementRetry(ctx context.Context, id int64) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockFailedNotifications) Delete(ctx context.Context, id int64) error {
	return m.Called(ctx, id).Error(0)
}

type mockFailedConfirmations struct{ mock.Mock }

func (m *mockFailedConfirmations) ListRetryable(
	ctx context.Context,
	maxRetries int,
	notBefore time.Time,
) ([]domain.FailedConfirmation, error) {
	args := m.Called(ctx, maxRetries, notBefore)
	v, _ := args.Get(0).([]domain.FailedConfirmation)

	return v, args.Error(1)
}

func (m *mockFailedConfirmations) ListExpired(
	ctx context.Context,
	cutoff time.Time,
) ([]domain.FailedConfirmation, error) {
	args := m.Called(ctx, cutoff)
	v, _ := args.Get(0).([]domain.FailedConfirmation)

	return v, args.Error(1)
}

func (m *mockFailedConfirmations) IncrementRetry(ctx context.Context, id int64) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockFailedConfirmations) Delete(ctx context.Context, id int64) error {
	return m.Called(ctx, id).Error(0)
}

type mockRecipients struct{ mock.Mock }

func (m *mockRecipients) Recipient(ctx context.Context, email, repoName string) (domain.Recipient, error) {
	args := m.Called(ctx, email, repoName)
	v, _ := args.Get(0).(domain.Recipient)

	return v, args.Error(1)
}

type mockReleaseSender struct{ mock.Mock }

func (m *mockReleaseSender) SendReleaseNotifications(
	ctx context.Context,
	notifications []notifier.ReleaseNotification,
) notifier.BatchResult {
	args := m.Called(ctx, notifications)
	v, _ := args.Get(0).(notifier.BatchResult)

	return v
}

type mockConfirmationSender struct{ mock.Mock }

func (m *mockConfirmationSender) DeliverConfirmation(ctx context.Context, to, repo, confirmURL string) error {
	return m.Called(ctx, to, repo, confirmURL).Error(0)
}

type mockURLs struct{ mock.Mock }

func (m *mockURLs) UnsubscribeURL(token string) string {
	return m.Called(token).String(0)
}

type mockPublisher struct{ mock.Mock }

func (m *mockPublisher) ReleaseSent(ctx context.Context, ev events.NotificationReleaseSent) error {
	return m.Called(ctx, ev).Error(0)
}

func (m *mockPublisher) ReleaseDead(ctx context.Context, ev events.NotificationReleaseDead) error {
	return m.Called(ctx, ev).Error(0)
}

func (m *mockPublisher) ConfirmationSent(ctx context.Context, ev events.NotificationConfirmationSent) error {
	return m.Called(ctx, ev).Error(0)
}

func (m *mockPublisher) ConfirmationDead(ctx context.Context, ev events.NotificationConfirmationDead) error {
	return m.Called(ctx, ev).Error(0)
}
