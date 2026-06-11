package grcp_api

import (
	"fmt"
	"log/slog"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/subscription/grpc/gen/appv1"
)

func NewGRPCServer(handler *Server, apiKey string, log *slog.Logger) (*grpc.Server, error) {
	validator, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("create validator: %w", err)
	}

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			logging.NewCorrelationInterceptor(),
			logging.NewSlogInterceptor(log),
			metrics.NewMetricsInterceptor,
			NewKeyAuthInterceptor(apiKey),
			NewValidationInterceptor(validator),
		),
	)

	appv1.RegisterSubscriptionServiceServer(srv, handler)

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, healthSrv)

	reflection.Register(srv)

	return srv, nil
}
