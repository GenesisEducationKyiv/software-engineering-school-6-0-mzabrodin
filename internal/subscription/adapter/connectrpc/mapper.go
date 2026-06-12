package connectapi

import (
	"github-release-notifier/internal/subscription/domain"
	"github-release-notifier/internal/subscription/grpc/gen/appv1"
)

func toProtoSubscription(view *domain.SubscriptionView) *appv1.Subscription {
	return &appv1.Subscription{
		Email:       view.Email,
		Repo:        view.Repo,
		Confirmed:   view.Confirmed,
		LastSeenTag: view.LastSeenTag,
	}
}

func toProtoSubscriptions(views []*domain.SubscriptionView) []*appv1.Subscription {
	subs := make([]*appv1.Subscription, len(views))
	for i, view := range views {
		subs[i] = toProtoSubscription(view)
	}

	return subs
}
