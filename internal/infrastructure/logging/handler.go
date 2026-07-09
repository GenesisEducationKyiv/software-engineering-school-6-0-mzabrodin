package logging

import (
	"context"
	"log/slog"
)

type fieldHandler struct {
	slog.Handler
	key     string
	extract func(context.Context) string
}

func (h fieldHandler) Handle(
	ctx context.Context,
	r slog.Record, //nolint:gocritic // slog.Handler interface requires value receiver; cannot pass slog.Record by pointer
) error {
	if v := h.extract(ctx); v != "" {
		r.AddAttrs(slog.String(h.key, v))
	}

	return h.Handler.Handle( //nolint:wrapcheck // implementing slog.Handler interface, callers (slog internals) discard this error anyway
		ctx,
		r,
	)
}

func (h fieldHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return fieldHandler{h.Handler.WithAttrs(attrs), h.key, h.extract}
}

func (h fieldHandler) WithGroup(name string) slog.Handler {
	return fieldHandler{h.Handler.WithGroup(name), h.key, h.extract}
}

func newFieldHandler(inner slog.Handler, key string, extract func(context.Context) string) slog.Handler {
	return fieldHandler{Handler: inner, key: key, extract: extract}
}
