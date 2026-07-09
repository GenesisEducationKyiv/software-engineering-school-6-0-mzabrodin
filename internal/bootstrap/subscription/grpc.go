package subscription

import (
	"net/http"

	"log/slog"

	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"

	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/shared/grpc/gen/compensationv1/compensationv1connect"
	"github-release-notifier/internal/subscription/adapter/compensationserver"
	"github-release-notifier/internal/subscription/usecase/compensate"
)

func newInternalGRPCServer(
	cfg *config.SubscriptionConfig,
	comp *compensate.UseCase,
	log *slog.Logger,
) (*http.Server, error) {
	svc := compensationserver.NewService(comp, log)

	path, handler, err := compensationserver.NewHandler(svc, log)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle(path, handler)

	healthPath, healthHandler := grpchealth.NewHandler(
		grpchealth.NewStaticChecker(compensationv1connect.CompensationServiceName),
	)
	mux.Handle(healthPath, healthHandler)

	reflector := grpcreflect.NewStaticReflector(compensationv1connect.CompensationServiceName)
	reflectV1Path, reflectV1Handler := grpcreflect.NewHandlerV1(reflector)
	reflectV1AlphaPath, reflectV1AlphaHandler := grpcreflect.NewHandlerV1Alpha(reflector)
	mux.Handle(reflectV1Path, reflectV1Handler)
	mux.Handle(reflectV1AlphaPath, reflectV1AlphaHandler)

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	return &http.Server{
		Addr: ":" + cfg.GRPCPort,
		// EdgeMiddleware injects request-id and protocol into ctx for the observability interceptor.
		Handler:           logging.EdgeMiddleware(mux),
		ReadHeaderTimeout: shutdownTimeout,
		Protocols:         protocols,
	}, nil
}
