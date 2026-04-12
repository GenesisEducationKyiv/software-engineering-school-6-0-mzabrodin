package api

import (
	"context"
	"encoding/json"
	"errors"
	"github-release-notifier/internal/domain"
	"github-release-notifier/internal/service"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type SubscriptionService interface {
	Subscribe(ctx context.Context, email, repo string) error
	Confirm(ctx context.Context, token string) error
	Unsubscribe(ctx context.Context, token string) error
	GetByEmail(ctx context.Context, email string) ([]*domain.SubscriptionView, error)
}

type Handler struct {
	service SubscriptionService
}

func NewHandler(svc SubscriptionService) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var req SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Repo = strings.TrimSpace(req.Repo)

	if req.Email == "" || req.Repo == "" {
		jsonErr(w, "email and repo are required", http.StatusBadRequest)
		return
	}

	err := h.service.Subscribe(r.Context(), req.Email, req.Repo)
	switch {
	case errors.Is(err, service.ErrInvalidRepo):
		jsonErr(w, "invalid repo format, expected owner/repo", http.StatusBadRequest)
	case errors.Is(err, service.ErrRepoNotFound):
		jsonErr(w, "repository not found on GitHub", http.StatusNotFound)
	case errors.Is(err, domain.ErrAlreadyExists):
		jsonErr(w, "email already subscribed to this repository", http.StatusConflict)
	case err != nil:
		slog.Error("subscribe failed", "error", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
	default:
		jsonOK(w, MessageResponse{Message: "subscription successful, confirmation email sent"})
	}
}

func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if len(token) != 64 {
		jsonErr(w, "invalid token", http.StatusBadRequest)
		return
	}

	err := h.service.Confirm(r.Context(), token)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		jsonErr(w, "token not found", http.StatusNotFound)
	case err != nil:
		slog.Error("confirm failed", "error", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
	default:
		jsonOK(w, MessageResponse{Message: "subscription confirmed successfully"})
	}
}

func (h *Handler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if len(token) != 64 {
		jsonErr(w, "invalid token", http.StatusBadRequest)
		return
	}

	err := h.service.Unsubscribe(r.Context(), token)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		jsonErr(w, "token not found", http.StatusNotFound)
	case err != nil:
		slog.Error("unsubscribe failed", "error", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
	default:
		jsonOK(w, MessageResponse{Message: "unsubscribed successfully"})
	}
}

func (h *Handler) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.URL.Query().Get("email"))
	if email == "" || !strings.Contains(email, "@") {
		jsonErr(w, "invalid email", http.StatusBadRequest)
		return
	}

	subs, err := h.service.GetByEmail(r.Context(), email)
	if err != nil {
		slog.Error("get subscriptions failed", "error", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	responses := toSubscriptionResponses(subs)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(responses); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}
