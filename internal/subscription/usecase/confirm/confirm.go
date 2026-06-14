package confirm

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/github"
)

const welcomeReleaseTimeout = 30 * time.Second

type subRepository interface {
	Confirm(ctx context.Context, token string) (*entity.Subscription, string, error)
}

type gitHubClient interface {
	GetLatestRelease(ctx context.Context, owner, repo string) (*entity.Release, error)
}

type releaseNotifier interface {
	Notify(ctx context.Context, subs []*entity.Subscription, repo *entity.Repository, release *entity.Release) error
}

type Input struct {
	Token string
}

type Output struct{}

type UseCase struct {
	subs     subRepository
	github   gitHubClient
	notifier releaseNotifier
	log      *slog.Logger
}

func New(subs subRepository, gh gitHubClient, notifier releaseNotifier, log *slog.Logger) *UseCase {
	return &UseCase{subs: subs, github: gh, notifier: notifier, log: log.With("component", "confirm")}
}

func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	sub, repoName, err := uc.subs.Confirm(ctx, in.Token)
	if err != nil {
		return Output{}, err
	}

	uc.log.InfoContext(ctx, "subscription confirmed")

	if sub != nil {
		uc.sendCurrentRelease(ctx, sub, repoName)
	}

	return Output{}, nil
}

func (uc *UseCase) sendCurrentRelease(ctx context.Context, sub *entity.Subscription, repoName string) {
	owner, name, err := github.ParseRepo(repoName)
	if err != nil {
		uc.log.WarnContext(ctx, "cannot parse repo for welcome release", "repo", repoName, "error", err)
		return
	}

	go func() {
		sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), welcomeReleaseTimeout)
		defer cancel()

		release, err := uc.github.GetLatestRelease(sendCtx, owner, name)
		if errors.Is(err, github.ErrNoRelease) {
			return
		}

		if err != nil {
			uc.log.WarnContext(sendCtx, "failed to fetch latest release on confirm", "repo", repoName, "error", err)
			return
		}

		repo := &entity.Repository{Name: repoName}
		if err := uc.notifier.Notify(sendCtx, []*entity.Subscription{sub}, repo, release); err != nil {
			uc.log.WarnContext(sendCtx, "failed to send welcome release email", "repo", repoName, "error", err)
		}
	}()
}
