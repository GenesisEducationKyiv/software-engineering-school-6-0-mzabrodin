package compensationserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"

	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/shared/grpc/gen/compensationv1"
	"github-release-notifier/internal/shared/grpc/gen/compensationv1/compensationv1connect"
	"github-release-notifier/internal/subscription/usecase/compensate"
)

type compensator interface {
	Execute(ctx context.Context, in compensate.Input) (bool, error)
}

type Service struct {
	compensate compensator
	log        *slog.Logger
}

func NewService(c compensator, log *slog.Logger) *Service {
	return &Service{compensate: c, log: log.With("component", "compensation-server")}
}

func (s *Service) Compensate(
	ctx context.Context,
	req *connect.Request[compensationv1.CompensateRequest],
) (*connect.Response[compensationv1.CompensateResponse], error) {
	ctx = logging.WithSagaID(ctx, req.Msg.GetSagaId())

	rolledBack, err := s.compensate.Execute(ctx, compensate.Input{
		Email:    req.Msg.GetEmail(),
		RepoName: req.Msg.GetRepoName(),
	})
	if err != nil {
		s.log.ErrorContext(ctx, "compensation failed",
			"error", err, "email", req.Msg.GetEmail(), "repo", req.Msg.GetRepoName())

		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&compensationv1.CompensateResponse{RolledBack: rolledBack}), nil
}

func NewHandler(svc *Service, log *slog.Logger) (string, http.Handler, error) {
	validator, err := protovalidate.New()
	if err != nil {
		return "", nil, fmt.Errorf("create validator: %w", err)
	}

	path, handler := compensationv1connect.NewCompensationServiceHandler(svc,
		connect.WithInterceptors(
			metrics.NewConnectObservabilityInterceptor(log, logging.Protocol),
			newValidationInterceptor(validator),
		),
	)

	return path, handler, nil
}
