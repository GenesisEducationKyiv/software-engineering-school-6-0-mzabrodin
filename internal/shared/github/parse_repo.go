package github

import (
	"regexp"
	"strings"
)

var repoRegex = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func ParseRepo(repo string) (owner, name string, err error) {
	if !repoRegex.MatchString(repo) {
		return "", "", ErrInvalidRepo
	}

	parts := strings.SplitN(repo, "/", 2)
	return parts[0], parts[1], nil
}
