package http_api

import (
	"errors"
	"net/http"

	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/github"
)

var domainErrorMappings = []struct {
	target  error
	status  int
	message string
}{
	{github.ErrInvalidRepo, http.StatusBadRequest, "invalid repo format, expected owner/repo"},
	{github.ErrRepoNotFound, http.StatusNotFound, "repository not found on GitHub"},
	{entity.ErrAlreadyExists, http.StatusConflict, "email already subscribed to this repository"},
	{entity.ErrNotFound, http.StatusNotFound, "token not found"},
}

func (h *Handler) writeDomainError(w http.ResponseWriter, r *http.Request, err error, logArgs ...any) {
	for _, m := range domainErrorMappings {
		if errors.Is(err, m.target) {
			jsonErr(w, m.message, m.status)
			return
		}
	}

	h.log.ErrorContext(r.Context(), "request failed", append([]any{"error", err}, logArgs...)...)
	jsonErr(w, "internal server error", http.StatusInternalServerError)
}
