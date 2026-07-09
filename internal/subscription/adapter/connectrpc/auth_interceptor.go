package connectapi

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	"github-release-notifier/internal/subscription/grpc/gen/subscriptionv1/subscriptionv1connect"
)

const (
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "
)

var protectedProcedures = map[string]bool{
	subscriptionv1connect.SubscriptionServiceSubscribeProcedure:         true,
	subscriptionv1connect.SubscriptionServiceListSubscriptionsProcedure: true,
}

func NewAuthInterceptor(apiKey string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if apiKey == "" || !protectedProcedures[req.Spec().Procedure] {
				return next(ctx, req)
			}

			if bearerToken(req.Header().Get(authorizationHeader)) != apiKey {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthorized"))
			}

			return next(ctx, req)
		}
	}
}

func bearerToken(header string) string {
	if len(header) < len(bearerPrefix) || !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return ""
	}

	return header[len(bearerPrefix):]
}
