package connectapi

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/shared/domain"
	"github-release-notifier/internal/shared/github"
)

type DomainErrorSuite struct {
	suite.Suite
	svc *Service
}

func TestDomainErrorSuite(t *testing.T) {
	suite.Run(t, new(DomainErrorSuite))
}

func (s *DomainErrorSuite) SetupTest() {
	s.svc = NewService(nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (s *DomainErrorSuite) TestMapsDomainErrorsToConnectCodes() {
	cases := []struct {
		name string
		err  error
		want connect.Code
	}{
		{"invalid repo", github.ErrInvalidRepo, connect.CodeInvalidArgument},
		{"repo not found", github.ErrRepoNotFound, connect.CodeNotFound},
		{"already exists", domain.ErrAlreadyExists, connect.CodeAlreadyExists},
		{"token not found", domain.ErrNotFound, connect.CodeNotFound},
		{"unmapped", errors.New("boom"), connect.CodeInternal},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			err := s.svc.domainError(s.T().Context(), tc.err, "email", "user@example.com")

			s.Equal(tc.want, connect.CodeOf(err))
		})
	}
}

func (s *DomainErrorSuite) TestWrappedDomainErrorIsMapped() {
	err := s.svc.domainError(s.T().Context(), errors.Join(domain.ErrNotFound, errors.New("ctx")), "k", "v")

	s.Equal(connect.CodeNotFound, connect.CodeOf(err))
}
