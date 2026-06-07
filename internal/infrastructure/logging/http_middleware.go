package logging

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func NewSlogMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			mw := &statusRecorder{ResponseWriter: w}
			start := time.Now()
			next.ServeHTTP(mw, r)
			status := mw.status

			if status == 0 {
				status = http.StatusOK
			}

			path := chi.RouteContext(r.Context()).RoutePattern()
			if path == "" {
				path = r.URL.Path
			}

			log.InfoContext(r.Context(), "http request",
				"method", r.Method,
				"path", path,
				"status_code", status,
				"duration_ms", time.Since(start).Milliseconds(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}
