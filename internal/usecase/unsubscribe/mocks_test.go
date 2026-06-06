package unsubscribe_test

import (
	"context"
	"io"
	"log/slog"

	"github.com/stretchr/testify/mock"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) Delete(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}
