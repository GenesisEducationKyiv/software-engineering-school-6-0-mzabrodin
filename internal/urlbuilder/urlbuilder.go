package urlbuilder

import "fmt"

type URLBuilder struct {
	baseURL string
}

func New(baseURL string) *URLBuilder {
	return &URLBuilder{baseURL: baseURL}
}

func (u *URLBuilder) ConfirmURL(token string) string {
	return fmt.Sprintf("%s/api/confirm/%s", u.baseURL, token)
}

func (u *URLBuilder) UnsubscribeURL(token string) string {
	return fmt.Sprintf("%s/api/unsubscribe/%s", u.baseURL, token)
}
