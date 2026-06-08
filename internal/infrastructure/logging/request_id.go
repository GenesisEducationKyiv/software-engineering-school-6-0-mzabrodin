package logging

import (
	"context"
	"log/slog"
)

type requestIDKeyType struct{}

var requestIDKey requestIDKeyType

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func NewRequestIDHandler(inner slog.Handler) slog.Handler {
	return newFieldHandler(inner, "request_id", RequestID)
}
