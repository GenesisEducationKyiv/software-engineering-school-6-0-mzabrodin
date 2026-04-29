package service

import "errors"

var (
	ErrInvalidRepo  = errors.New("invalid repository format, expected owner/repo")
	ErrRepoNotFound = errors.New("repository not found on GitHub")
)
