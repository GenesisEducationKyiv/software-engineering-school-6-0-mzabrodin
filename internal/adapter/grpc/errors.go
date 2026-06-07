package grcp_api

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github-release-notifier/internal/entity"
)

var domainErrorMappings = []struct {
	target  error
	code    codes.Code
	message string
}{
	{entity.ErrInvalidRepo, codes.InvalidArgument, "invalid repo format, expected owner/repo"},
	{entity.ErrRepoNotFound, codes.NotFound, "repository not found on GitHub"},
	{entity.ErrAlreadyExists, codes.AlreadyExists, "email already subscribed to this repository"},
	{entity.ErrNotFound, codes.NotFound, "token not found"},
}

func (s *Server) writeDomainError(ctx context.Context, err error, logArgs ...any) error {
	for _, m := range domainErrorMappings {
		if errors.Is(err, m.target) {
			return status.Error(m.code, m.message)
		}
	}

	s.log.ErrorContext(ctx, "request failed", append([]any{"error", err}, logArgs...)...)

	return status.Error(codes.Internal, "internal server error")
}
