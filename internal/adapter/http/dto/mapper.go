package dto

import "github-release-notifier/internal/entity"

func ToSubscriptionResponse(view *entity.SubscriptionView) SubscriptionResponse {
	return SubscriptionResponse{
		Email:       view.Email,
		Repo:        view.Repo,
		Confirmed:   view.Confirmed,
		LastSeenTag: view.LastSeenTag,
	}
}

func ToSubscriptionResponses(views []*entity.SubscriptionView) []SubscriptionResponse {
	responses := make([]SubscriptionResponse, 0, len(views))
	for _, v := range views {
		responses = append(responses, ToSubscriptionResponse(v))
	}

	return responses
}
