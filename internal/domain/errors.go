package domain

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrRateLimited   = errors.New("github API rate limit exceeded")
	ErrUnauthorized  = errors.New("github API unauthorized: check GITHUB_TOKEN")
	ErrNoRelease     = errors.New("no releases found")
)
