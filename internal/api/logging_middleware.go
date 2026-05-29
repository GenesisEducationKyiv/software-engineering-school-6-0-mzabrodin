package api

import (
	"log/slog"
	"net/http"
	"time"
)

func NewSlogMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mw := &metricsWriter{ResponseWriter: w}
			start := time.Now()
			next.ServeHTTP(mw, r)
			status := mw.status

			if status == 0 {
				status = http.StatusOK
			}

			log.InfoContext(r.Context(), "http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status_code", status,
				"duration_ms", time.Since(start).Milliseconds(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}
