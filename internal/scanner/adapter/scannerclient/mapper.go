package scannerclient

import (
	"github-release-notifier/internal/scanner/grpc/gen/scannerv1"
	"github-release-notifier/internal/shared/entity"
)

func toObservedReleases(in []*scannerv1.ObservedRelease) []entity.ObservedRelease {
	out := make([]entity.ObservedRelease, len(in))
	for i, rel := range in {
		out[i] = entity.ObservedRelease{
			Repo:    rel.GetRepo(),
			Release: &entity.Release{TagName: rel.GetTag(), HTMLURL: rel.GetReleaseUrl()},
		}
	}

	return out
}
