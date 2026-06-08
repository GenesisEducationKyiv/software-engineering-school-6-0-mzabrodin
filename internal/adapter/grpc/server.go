package grcp_api

import (
	"fmt"
	"log/slog"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	notifierv1 "github-release-notifier/internal/adapter/grpc/gen/app/v1"
	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/infrastructure/metrics"
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

	notifierv1.RegisterSubscriptionServiceServer(srv, handler)

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, healthSrv)

	reflection.Register(srv)

	return srv, nil
}
