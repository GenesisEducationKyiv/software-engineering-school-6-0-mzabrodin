package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type metricsWriter struct {
	http.ResponseWriter
	status int
}

func (mw *metricsWriter) WriteHeader(code int) {
	mw.status = code
	mw.ResponseWriter.WriteHeader(code)
}

func (mw *metricsWriter) Write(b []byte) (int, error) {
	if mw.status == 0 {
		mw.status = http.StatusOK
	}

	n, err := mw.ResponseWriter.Write(b)
	if err != nil {
		return n, fmt.Errorf("write response: %w", err)
	}

	return n, nil
}

func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		mw := &metricsWriter{ResponseWriter: w}
		start := time.Now()

		next.ServeHTTP(mw, r)

		path := chi.RouteContext(r.Context()).RoutePattern()
		if path == "" {
			path = r.URL.Path
		}

		status := mw.status
		if status == 0 {
			status = http.StatusOK
		}

		HTTPRequestsTotal.WithLabelValues(
			r.Method,
			path,
			fmt.Sprintf("%d", status),
		).Inc()

		HTTPRequestDuration.WithLabelValues(
			r.Method,
			path,
		).Observe(time.Since(start).Seconds())
	})
}
