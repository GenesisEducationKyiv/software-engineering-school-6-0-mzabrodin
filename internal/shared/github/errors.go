package github

import "errors"

var (
	ErrRateLimited  = errors.New("github API rate limit exceeded")
	ErrUnauthorized = errors.New("github API unauthorized: check GITHUB_TOKEN")
	ErrNoRelease    = errors.New("no releases found")
	ErrInvalidRepo  = errors.New("invalid repository format, expected owner/repo")
	ErrRepoNotFound = errors.New("repository not found on GitHub")
)
