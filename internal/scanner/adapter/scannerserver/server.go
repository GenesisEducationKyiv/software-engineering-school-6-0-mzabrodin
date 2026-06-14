package scannerserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"google.golang.org/protobuf/proto"

	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/scanner/grpc/gen/scannerv1/scannerv1connect"
)

func NewHandler(handler scannerv1connect.ScannerServiceHandler, log *slog.Logger) (http.Handler, error) {
	validator, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("create validator: %w", err)
	}

	path, svc := scannerv1connect.NewScannerServiceHandler(handler,
		connect.WithInterceptors(
			metrics.NewConnectObservabilityInterceptor(log, func(context.Context) string { return "grpc" }),
			logging.NewConnectCorrelationInterceptor(),
			newValidationInterceptor(validator),
		),
	)

	mux := http.NewServeMux()
	mux.Handle(path, svc)

	healthPath, healthHandler := grpchealth.NewHandler(
		grpchealth.NewStaticChecker(scannerv1connect.ScannerServiceName),
	)
	mux.Handle(healthPath, healthHandler)

	reflector := grpcreflect.NewStaticReflector(scannerv1connect.ScannerServiceName)
	reflectV1Path, reflectV1Handler := grpcreflect.NewHandlerV1(reflector)
	reflectV1AlphaPath, reflectV1AlphaHandler := grpcreflect.NewHandlerV1Alpha(reflector)
	mux.Handle(reflectV1Path, reflectV1Handler)
	mux.Handle(reflectV1AlphaPath, reflectV1AlphaHandler)

	return mux, nil
}

func newValidationInterceptor(validator protovalidate.Validator) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if msg, ok := req.Any().(proto.Message); ok {
				if err := validator.Validate(msg); err != nil {
					return nil, connect.NewError(connect.CodeInvalidArgument, errors.New(err.Error()))
				}
			}

			return next(ctx, req)
		}
	}
}
