package grcp_api

import (
	notifierv1 "github-release-notifier/internal/adapter/grpc/gen/app/v1"
	"github-release-notifier/internal/entity"
)

func toProtoSubscription(view *entity.SubscriptionView) *notifierv1.Subscription {
	return &notifierv1.Subscription{
		Email:       view.Email,
		Repo:        view.Repo,
		Confirmed:   view.Confirmed,
		LastSeenTag: view.LastSeenTag,
	}
}

func toProtoSubscriptions(views []*entity.SubscriptionView) []*notifierv1.Subscription {
	subs := make([]*notifierv1.Subscription, len(views))
	for i, view := range views {
		subs[i] = toProtoSubscription(view)
	}

	return subs
}
