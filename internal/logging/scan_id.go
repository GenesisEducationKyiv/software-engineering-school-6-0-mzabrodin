package logging

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

var scanIDKey ctxKey

func WithScanID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, scanIDKey, id)
}

func ScanID(ctx context.Context) string {
	id, _ := ctx.Value(scanIDKey).(string)
	return id
}

func NewScanIDHandler(inner slog.Handler) slog.Handler {
	return newFieldHandler(inner, "scan_id", ScanID)
}
