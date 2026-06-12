package logging

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

const (
	requestIDMetaKey = "x-request-id"
	scanIDMetaKey    = "x-scan-id"
)

func NewConnectCorrelationInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if req.Spec().IsClient {
				attachOutgoingIDs(ctx, req)
			} else {
				ctx = readIncomingIDs(ctx, req)
			}

			return next(ctx, req)
		}
	}
}

func attachOutgoingIDs(ctx context.Context, req connect.AnyRequest) {
	if id := RequestID(ctx); id != "" {
		req.Header().Set(requestIDMetaKey, id)
	}

	if id := ScanID(ctx); id != "" {
		req.Header().Set(scanIDMetaKey, id)
	}
}

func readIncomingIDs(ctx context.Context, req connect.AnyRequest) context.Context {
	id := req.Header().Get(requestIDMetaKey)
	if id == "" {
		id = uuid.NewString()
	}

	ctx = WithRequestID(ctx, id)

	if scanID := req.Header().Get(scanIDMetaKey); scanID != "" {
		ctx = WithScanID(ctx, scanID)
	}

	return ctx
}
