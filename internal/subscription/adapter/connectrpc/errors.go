package connectapi

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/github"
	"github-release-notifier/internal/subscription/adapter/confirmtoken"
)

var domainErrorMappings = []struct {
	target  error
	code    connect.Code
	message string
}{
	{github.ErrInvalidRepo, connect.CodeInvalidArgument, "invalid repo format, expected owner/repo"},
	{github.ErrRepoNotFound, connect.CodeNotFound, "repository not found on GitHub"},
	{entity.ErrAlreadyExists, connect.CodeAlreadyExists, "email already subscribed to this repository"},
	{entity.ErrNotFound, connect.CodeNotFound, "token not found"},
	{confirmtoken.ErrExpired, connect.CodeNotFound, "confirmation link has expired"},
	{confirmtoken.ErrInvalid, connect.CodeInvalidArgument, "invalid confirmation token"},
}

func (s *Service) domainError(ctx context.Context, err error, logArgs ...any) error {
	for _, m := range domainErrorMappings {
		if errors.Is(err, m.target) {
			return connect.NewError(m.code, errors.New(m.message))
		}
	}

	s.log.ErrorContext(ctx, "request failed", append([]any{"error", err}, logArgs...)...)

	return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
}
