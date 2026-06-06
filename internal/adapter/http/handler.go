package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github-release-notifier/internal/adapter/http/dto"
	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/usecase/confirm"
	"github-release-notifier/internal/usecase/list"
	"github-release-notifier/internal/usecase/subscribe"
	"github-release-notifier/internal/usecase/unsubscribe"
)

const tokenHexLen = 64

type subscriber interface {
	Execute(ctx context.Context, in subscribe.Input) (subscribe.Output, error)
}

type confirmer interface {
	Execute(ctx context.Context, in confirm.Input) (confirm.Output, error)
}

type unsubscriber interface {
	Execute(ctx context.Context, in unsubscribe.Input) (unsubscribe.Output, error)
}

type lister interface {
	Execute(ctx context.Context, in list.Input) (list.Output, error)
}

type Handler struct {
	subscribe   subscriber
	confirm     confirmer
	unsubscribe unsubscriber
	list        lister
	log         *slog.Logger
}

func NewHandler(
	sub subscriber,
	conf confirmer,
	unsub unsubscriber,
	lst lister,
	log *slog.Logger,
) *Handler {
	return &Handler{
		subscribe:   sub,
		confirm:     conf,
		unsubscribe: unsub,
		list:        lst,
		log:         log.With("component", "handler"),
	}
}

func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var req dto.SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Repo = strings.TrimSpace(req.Repo)

	if err := dto.Validate(req); err != nil {
		jsonErr(w, dto.ValidationMessage(err), http.StatusBadRequest)
		return
	}

	_, err := h.subscribe.Execute(r.Context(), subscribe.Input{Email: req.Email, Repo: req.Repo})
	switch {
	case errors.Is(err, entity.ErrInvalidRepo):
		jsonErr(w, "invalid repo format, expected owner/repo", http.StatusBadRequest)

	case errors.Is(err, entity.ErrRepoNotFound):
		jsonErr(w, "repository not found on GitHub", http.StatusNotFound)

	case errors.Is(err, entity.ErrAlreadyExists):
		jsonErr(w, "email already subscribed to this repository", http.StatusConflict)

	case err != nil:
		h.log.ErrorContext(r.Context(), "subscribe failed", "email", req.Email, "repo", req.Repo, "error", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)

	default:
		jsonOK(w, dto.MessageResponse{Message: "subscription successful, confirmation email sent"})
	}
}

func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if len(token) != tokenHexLen {
		jsonErr(w, "invalid token", http.StatusBadRequest)
		return
	}

	_, err := h.confirm.Execute(r.Context(), confirm.Input{Token: token})
	switch {
	case errors.Is(err, entity.ErrNotFound):
		jsonErr(w, "token not found", http.StatusNotFound)

	case err != nil:
		h.log.ErrorContext(r.Context(), "confirm failed", "token", token, "error", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)

	default:
		jsonOK(w, dto.MessageResponse{Message: "subscription confirmed successfully"})
	}
}

func (h *Handler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if len(token) != tokenHexLen {
		jsonErr(w, "invalid token", http.StatusBadRequest)
		return
	}

	_, err := h.unsubscribe.Execute(r.Context(), unsubscribe.Input{Token: token})
	switch {
	case errors.Is(err, entity.ErrNotFound):
		jsonErr(w, "token not found", http.StatusNotFound)

	case err != nil:
		h.log.ErrorContext(r.Context(), "unsubscribe failed", "token", token, "error", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)

	default:
		jsonOK(w, dto.MessageResponse{Message: "unsubscribed successfully"})
	}
}

func (h *Handler) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.URL.Query().Get("email"))
	if err := dto.ValidateEmail(email); err != nil {
		jsonErr(w, "invalid email format", http.StatusBadRequest)
		return
	}

	out, err := h.list.Execute(r.Context(), list.Input{Email: email})
	switch {
	case err != nil:
		h.log.ErrorContext(r.Context(), "get subscriptions failed", "email", email, "error", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)

	default:
		jsonOK(w, dto.ToSubscriptionResponses(out.Views))
	}
}
