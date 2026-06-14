package subscription

import (
	"fmt"
	"log/slog"
	"net/http"

	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"connectrpc.com/vanguard"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github-release-notifier/internal/infrastructure/logging"
	connectapi "github-release-notifier/internal/subscription/adapter/connectrpc"
	"github-release-notifier/internal/subscription/grpc/gen/subscriptionv1/subscriptionv1connect"
)

func NewHandler(svc *connectapi.Service, apiKey string, log *slog.Logger) (http.Handler, error) {
	_, connectHandler, err := connectapi.NewConnectHandler(svc, apiKey, log)
	if err != nil {
		return nil, err
	}

	transcoder, err := vanguard.NewTranscoder([]*vanguard.Service{
		vanguard.NewService(
			subscriptionv1connect.SubscriptionServiceName,
			connectHandler,
			vanguard.WithTargetCodecs(vanguard.CodecProto),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("create transcoder: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", transcoder)

	healthPath, healthHandler := grpchealth.NewHandler(
		grpchealth.NewStaticChecker(subscriptionv1connect.SubscriptionServiceName),
	)
	mux.Handle(healthPath, healthHandler)

	reflector := grpcreflect.NewStaticReflector(subscriptionv1connect.SubscriptionServiceName)
	reflectV1Path, reflectV1Handler := grpcreflect.NewHandlerV1(reflector)
	reflectV1AlphaPath, reflectV1AlphaHandler := grpcreflect.NewHandlerV1Alpha(reflector)
	mux.Handle(reflectV1Path, reflectV1Handler)
	mux.Handle(reflectV1AlphaPath, reflectV1AlphaHandler)

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/metrics", promhttp.Handler())

	return logging.EdgeMiddleware(mux), nil
}
