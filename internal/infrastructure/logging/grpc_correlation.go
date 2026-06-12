package logging

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	requestIDMetaKey = "x-request-id"
	scanIDMetaKey    = "x-scan-id"
)

func NewCorrelationInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		id := metadataValue(ctx, requestIDMetaKey)
		if id == "" {
			id = uuid.NewString()
		}

		ctx = WithRequestID(ctx, id)
		_ = grpc.SetHeader(ctx, metadata.Pairs(requestIDMetaKey, id))

		if scanID := metadataValue(ctx, scanIDMetaKey); scanID != "" {
			ctx = WithScanID(ctx, scanID)
		}

		return handler(ctx, req)
	}
}

func WithOutgoingIDs(ctx context.Context) context.Context {
	if id := RequestID(ctx); id != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, requestIDMetaKey, id)
	}

	if id := ScanID(ctx); id != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, scanIDMetaKey, id)
	}

	return ctx
}

func metadataValue(ctx context.Context, key string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}
