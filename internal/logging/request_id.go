package logging

import (
	"log/slog"

	"github.com/go-chi/chi/v5/middleware"
)

func NewRequestIDHandler(inner slog.Handler) slog.Handler {
	return newFieldHandler(inner, "request_id", middleware.GetReqID)
}
