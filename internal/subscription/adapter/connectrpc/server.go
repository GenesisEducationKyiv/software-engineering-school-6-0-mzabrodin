package connectapi

import (
	"fmt"
	"log/slog"
	"net/http"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"

	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/subscription/grpc/gen/subscriptionv1/subscriptionv1connect"
)

func NewConnectHandler(svc *Service, apiKey string, log *slog.Logger) (string, http.Handler, error) {
	validator, err := protovalidate.New()
	if err != nil {
		return "", nil, fmt.Errorf("create validator: %w", err)
	}

	path, handler := subscriptionv1connect.NewSubscriptionServiceHandler(svc,
		connect.WithInterceptors(
			metrics.NewConnectObservabilityInterceptor(log, logging.Protocol),
			NewAuthInterceptor(apiKey),
			NewValidationInterceptor(validator),
		),
	)

	return path, handler, nil
}
