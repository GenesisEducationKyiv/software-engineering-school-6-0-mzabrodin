package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/infrastructure/metrics"
)

func NewRouter(h *Handler, apiKey string, log *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, map[string]string{"status": "ok"})
	})

	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	r.Route("/api", func(r chi.Router) {
		r.Use(logging.NewSlogMiddleware(log))
		r.Use(metrics.HTTPMiddleware)

		r.Get("/confirm/{token}", h.Confirm)
		r.Get("/unsubscribe/{token}", h.Unsubscribe)

		r.Group(func(r chi.Router) {
			r.Use(KeyAuth(apiKey))
			r.Post("/subscribe", h.Subscribe)
			r.Get("/subscriptions", h.GetSubscriptions)
		})
	})

	return r
}
