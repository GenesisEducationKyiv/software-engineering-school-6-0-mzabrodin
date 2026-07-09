package advancetag

import (
	"context"
	"fmt"
	"log/slog"
)

type repository interface {
	AdvanceTag(ctx context.Context, repoName, tag string) error
}

type Input struct {
	RepoName  string
	Tag       string
	SentCount int
}

type UseCase struct {
	repos repository
	log   *slog.Logger
}

func New(repos repository, log *slog.Logger) *UseCase {
	return &UseCase{repos: repos, log: log.With("component", "advance-tag")}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) error {
	if in.SentCount <= 0 {
		uc.log.InfoContext(ctx, "release not delivered to anyone; not advancing tag",
			"repo", in.RepoName, "tag", in.Tag)

		return nil
	}

	if err := uc.repos.AdvanceTag(ctx, in.RepoName, in.Tag); err != nil {
		return fmt.Errorf("advance tag: %w", err)
	}

	uc.log.InfoContext(ctx, "advanced last seen tag", "repo", in.RepoName, "tag", in.Tag, "sent", in.SentCount)

	return nil
}
