package api

import "github-release-notifier/internal/entity"

type SubscribeRequest struct {
	Email string `json:"email"`
	Repo  string `json:"repo"`
}

type SubscriptionResponse struct {
	Email       string  `json:"email"`
	Repo        string  `json:"repo"`
	Confirmed   bool    `json:"confirmed"`
	LastSeenTag *string `json:"last_seen_tag,omitempty"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func toSubscriptionResponse(view *entity.SubscriptionView) SubscriptionResponse {
	return SubscriptionResponse{
		Email:       view.Email,
		Repo:        view.Repo,
		Confirmed:   view.Confirmed,
		LastSeenTag: view.LastSeenTag,
	}
}

func toSubscriptionResponses(views []*entity.SubscriptionView) []SubscriptionResponse {
	if views == nil {
		return []SubscriptionResponse{}
	}

	responses := make([]SubscriptionResponse, len(views))
	for i, v := range views {
		responses[i] = toSubscriptionResponse(v)
	}

	return responses
}
