package dto

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
