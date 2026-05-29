package api

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5/middleware"
)

type ctxHandler struct{ slog.Handler }

func (h ctxHandler) Handle(
	ctx context.Context,
	r slog.Record, //nolint:gocritic // slog.Handler interface requires value receiver; cannot pass slog.Record by pointer
) error {
	if id := middleware.GetReqID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}

	return h.Handler.Handle( //nolint:wrapcheck // implementing slog.Handler interface, callers (slog internals) discard this error anyway
		ctx,
		r,
	)
}

func (h ctxHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ctxHandler{h.Handler.WithAttrs(attrs)}
}

func (h ctxHandler) WithGroup(name string) slog.Handler {
	return ctxHandler{h.Handler.WithGroup(name)}
}

func NewContextHandler(inner slog.Handler) slog.Handler {
	return ctxHandler{inner}
}
