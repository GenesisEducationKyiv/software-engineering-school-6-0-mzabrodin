package api_test

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github-release-notifier/internal/usecase/confirm"
	"github-release-notifier/internal/usecase/list"
	"github-release-notifier/internal/usecase/subscribe"
	"github-release-notifier/internal/usecase/unsubscribe"
)

type mockSubscribe struct{ mock.Mock }

func (m *mockSubscribe) Execute(ctx context.Context, in subscribe.Input) (subscribe.Output, error) {
	args := m.Called(ctx, in)
	out, _ := args.Get(0).(subscribe.Output)
	return out, args.Error(1)
}

type mockConfirm struct{ mock.Mock }

func (m *mockConfirm) Execute(ctx context.Context, in confirm.Input) (confirm.Output, error) {
	args := m.Called(ctx, in)
	out, _ := args.Get(0).(confirm.Output)
	return out, args.Error(1)
}

type mockUnsubscribe struct{ mock.Mock }

func (m *mockUnsubscribe) Execute(ctx context.Context, in unsubscribe.Input) (unsubscribe.Output, error) {
	args := m.Called(ctx, in)
	out, _ := args.Get(0).(unsubscribe.Output)
	return out, args.Error(1)
}

type mockList struct{ mock.Mock }

func (m *mockList) Execute(ctx context.Context, in list.Input) (list.Output, error) {
	args := m.Called(ctx, in)
	out, _ := args.Get(0).(list.Output)
	return out, args.Error(1)
}
