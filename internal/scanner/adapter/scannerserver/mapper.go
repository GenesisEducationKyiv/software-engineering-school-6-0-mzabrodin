package scannerserver

import (
	"github-release-notifier/internal/scanner/grpc/gen/scannerv1"
	"github-release-notifier/internal/shared/entity"
)

func toProtoObservedReleases(in []entity.ObservedRelease) []*scannerv1.ObservedRelease {
	out := make([]*scannerv1.ObservedRelease, len(in))
	for i, rel := range in {
		out[i] = &scannerv1.ObservedRelease{
			Repo:       rel.Repo,
			Tag:        rel.Release.TagName,
			ReleaseUrl: rel.Release.HTMLURL,
		}
	}

	return out
}
