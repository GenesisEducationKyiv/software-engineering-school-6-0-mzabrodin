package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github-release-notifier/internal/metrics"
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
	return mw.ResponseWriter.Write(b)
}

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		metrics.HTTPRequestsTotal.WithLabelValues(
			r.Method,
			path,
			fmt.Sprintf("%d", status),
		).Inc()

		metrics.HTTPRequestDuration.WithLabelValues(
			r.Method,
			path,
		).Observe(time.Since(start).Seconds())
	})
}
