package dto

type SubscribeRequest struct {
	Email string `json:"email" validate:"required,email"`
	Repo  string `json:"repo"  validate:"required,reponame"`
}
