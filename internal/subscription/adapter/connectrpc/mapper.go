package connectapi

import (
	"github-release-notifier/internal/subscription/domain"
	"github-release-notifier/internal/subscription/grpc/gen/subscriptionv1"
)

func toProtoSubscription(view *domain.SubscriptionView) *subscriptionv1.Subscription {
	return &subscriptionv1.Subscription{
		Email:       view.Email,
		Repo:        view.Repo,
		Confirmed:   view.Confirmed,
		LastSeenTag: view.LastSeenTag,
	}
}

func toProtoSubscriptions(views []*domain.SubscriptionView) []*subscriptionv1.Subscription {
	subs := make([]*subscriptionv1.Subscription, len(views))
	for i, view := range views {
		subs[i] = toProtoSubscription(view)
	}

	return subs
}
