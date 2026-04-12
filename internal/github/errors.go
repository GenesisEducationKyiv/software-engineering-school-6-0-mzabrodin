package github

import "errors"

var (
	ErrRateLimited = errors.New("github API rate limit exceeded")
	ErrNoRelease   = errors.New("no releases found")
)
