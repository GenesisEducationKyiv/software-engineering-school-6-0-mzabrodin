package logging

import (
	"context"
	"log/slog"
)

type sagaCtxKey struct{}

var sagaIDKey sagaCtxKey

func WithSagaID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sagaIDKey, id)
}

func SagaID(ctx context.Context) string {
	id, _ := ctx.Value(sagaIDKey).(string)
	return id
}

func NewSagaIDHandler(inner slog.Handler) slog.Handler {
	return newFieldHandler(inner, "saga_id", SagaID)
}
