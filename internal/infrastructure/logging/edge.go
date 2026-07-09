package logging

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-Id"

type protocolKeyType struct{}

var protocolKey protocolKeyType

func WithProtocol(ctx context.Context, protocol string) context.Context {
	return context.WithValue(ctx, protocolKey, protocol)
}

func Protocol(ctx context.Context) string {
	p, _ := ctx.Value(protocolKey).(string)
	return p
}

func DetectProtocol(r *http.Request) string {
	ct := r.Header.Get("Content-Type")

	switch {
	case strings.HasPrefix(ct, "application/grpc-web"):
		return "grpc-web"
	case strings.HasPrefix(ct, "application/grpc"):
		return "grpc"
	case strings.HasPrefix(r.URL.Path, "/api/"):
		return "rest"
	default:
		return "connect"
	}
}

func EdgeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}

		w.Header().Set(requestIDHeader, id)

		ctx := WithRequestID(r.Context(), id)
		ctx = WithProtocol(ctx, DetectProtocol(r))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
