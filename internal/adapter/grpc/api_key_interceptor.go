package grcp_api

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	notifierv1 "github-release-notifier/internal/adapter/grpc/gen/app/v1"
)

const apiKeyMetadataKey = "x-api-key"

var protectedMethods = map[string]bool{
	notifierv1.SubscriptionService_Subscribe_FullMethodName:         true,
	notifierv1.SubscriptionService_ListSubscriptions_FullMethodName: true,
}

func NewKeyAuthInterceptor(apiKey string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if apiKey == "" || !protectedMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		if !validAPIKey(ctx, apiKey) {
			return nil, status.Error(codes.Unauthenticated, "unauthorized")
		}

		return handler(ctx, req)
	}
}

func validAPIKey(ctx context.Context, apiKey string) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}

	values := md.Get(apiKeyMetadataKey)

	return len(values) > 0 && values[0] == apiKey
}
