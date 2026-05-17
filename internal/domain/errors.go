package domain

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrRateLimited   = errors.New("github API rate limit exceeded")
	ErrUnauthorized  = errors.New("github API unauthorized: check GITHUB_TOKEN")
	ErrNoRelease     = errors.New("no releases found")
	ErrInvalidRepo   = errors.New("invalid repository format, expected owner/repo")
	ErrRepoNotFound  = errors.New("repository not found on GitHub")
	ErrInvalidEmail  = errors.New("invalid email format")
)
