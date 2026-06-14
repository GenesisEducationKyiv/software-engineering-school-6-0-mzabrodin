package scannerserver

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	"github-release-notifier/internal/scanner/grpc/gen/scannerv1"
	"github-release-notifier/internal/shared/entity"
)

type scanner interface {
	Scan(ctx context.Context, repos []string) ([]entity.ObservedRelease, error)
}

type Server struct {
	scanner scanner
	log     *slog.Logger
}

func NewServer(sc scanner, log *slog.Logger) *Server {
	return &Server{scanner: sc, log: log.With("component", "scanner-handler")}
}

func (s *Server) Scan(
	ctx context.Context,
	req *connect.Request[scannerv1.ScanRequest],
) (*connect.Response[scannerv1.ScanResponse], error) {
	observed, err := s.scanner.Scan(ctx, req.Msg.GetRepositories())
	if err != nil {
		s.log.ErrorContext(ctx, "failed to scan repositories", "error", err)

		return nil, connect.NewError(connect.CodeInternal, errors.New("scan repositories"))
	}

	return connect.NewResponse(&scannerv1.ScanResponse{
		Releases: toProtoObservedReleases(observed),
	}), nil
}
