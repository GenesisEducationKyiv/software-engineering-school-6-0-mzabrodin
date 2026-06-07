package metrics

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func NewMetricsInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	start := time.Now()

	resp, err := handler(ctx, req)

	GRPCRequestsTotal.WithLabelValues(info.FullMethod, status.Code(err).String()).Inc()
	GRPCRequestDuration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())

	return resp, err
}
