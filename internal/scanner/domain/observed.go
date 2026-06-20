package domain

import "github-release-notifier/internal/shared/entity"

type ObservedRelease struct {
	Repo    string
	Release *entity.Release
}
