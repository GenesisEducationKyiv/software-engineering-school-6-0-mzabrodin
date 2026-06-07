package metrics_test

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type mockUseCase struct{ mock.Mock }

func (m *mockUseCase) Execute(ctx context.Context, in string) (string, error) {
	args := m.Called(ctx, in)

	return args.String(0), args.Error(1)
}
