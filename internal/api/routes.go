package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(h *Handler, apiKey string) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(KeyAuth(apiKey))
		r.Post("/subscribe", h.Subscribe)
		r.Get("/confirm/{token}", h.Confirm)
		r.Get("/unsubscribe/{token}", h.Unsubscribe)
		r.Get("/subscriptions", h.GetSubscriptions)
	})

	return r
}
