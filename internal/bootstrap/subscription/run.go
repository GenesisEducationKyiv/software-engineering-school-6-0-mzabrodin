package subscription

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/infrastructure/outbox"
	"github-release-notifier/internal/infrastructure/urlbuilder"
	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/shared/github"
	"github-release-notifier/internal/subscription/adapter/confirmtoken"
	connectapi "github-release-notifier/internal/subscription/adapter/connectrpc"
	"github-release-notifier/internal/subscription/adapter/eventpublisher"
	"github-release-notifier/internal/subscription/adapter/repository"
	submigrations "github-release-notifier/internal/subscription/migrations"
	"github-release-notifier/internal/subscription/usecase/cleanup"
	"github-release-notifier/internal/subscription/usecase/confirm"
	"github-release-notifier/internal/subscription/usecase/list"
	"github-release-notifier/internal/subscription/usecase/subscribe"
	"github-release-notifier/internal/subscription/usecase/unsubscribe"
)

const (
	shutdownTimeout = bootstrap.ShutdownTimeout
	relayInterval   = 5 * time.Second
	relayBatchSize  = 100
)

func Run(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	inf, cleanupInfra, err := newInfrastructure(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer cleanupInfra()

	svc, cleanupUC := buildApp(inf, cfg, log)

	publicSrv, err := newPublicServer(cfg, svc, log)
	if err != nil {
		return err
	}

	backgroundDone, cancelBackground := startBackground(ctx, inf.relay, cleanupUC, cfg.PendingCleanupInterval, log)

	return serve(ctx, serveDeps{
		publicSrv:        publicSrv,
		backgroundDone:   backgroundDone,
		cancelBackground: cancelBackground,
		log:              log,
	})
}

type infrastructure struct {
	pool  *pgxpool.Pool
	gh    *github.Client
	relay *outbox.Relay
}

func newInfrastructure(ctx context.Context, cfg *config.Config, log *slog.Logger) (*infrastructure, func(), error) {
	var closers []func()
	cleanupClosers := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}
	}

	ok := false
	defer func() {
		if !ok {
			cleanupClosers()
		}
	}()

	if err := db.RunMigrationsFS(cfg.DatabaseURL, submigrations.FS, log); err != nil {
		return nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	if err := db.RunMigrationsFS(
		cfg.DatabaseURL, outbox.Migrations, log, db.WithMigrationsTable("outbox_schema_migrations"),
	); err != nil {
		return nil, nil, fmt.Errorf("run outbox migrations: %w", err)
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL, metrics.NewPgxTracer(), log)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to database: %w", err)
	}
	closers = append(closers, pool.Close)

	redisCache, closeRedis, err := bootstrap.ConnectRedis(ctx, cfg.RedisURL, log)
	if err != nil {
		return nil, nil, err
	}
	closers = append(closers, closeRedis)

	gh := github.NewClient(cfg.GitHubToken, log).WithCache(redisCache, 10*time.Minute)

	brokerConn, closeBroker, err := bootstrap.ConnectBroker(
		ctx, cfg.NATSURL, events.StreamSubscriptions, events.SubjectsSubscriptions, log,
	)
	if err != nil {
		return nil, nil, err
	}
	closers = append(closers, closeBroker)

	if err := bootstrap.EnsureEventStreams(ctx, brokerConn); err != nil {
		return nil, nil, err
	}

	relay := outbox.NewRelay(pool, brokerConn, relayInterval, relayBatchSize, log)

	ok = true

	return &infrastructure{pool: pool, gh: gh, relay: relay}, cleanupClosers, nil
}

func buildApp(inf *infrastructure, cfg *config.Config, log *slog.Logger) (*connectapi.Service, *cleanup.UseCase) {
	repos := repository.NewGitHubRepoRepository(inf.pool)
	subs := repository.NewSubscriptionRepository(inf.pool)
	urls := urlbuilder.New(cfg.BaseURL)
	transactor := db.NewTransactor(inf.pool)
	pub := eventpublisher.New(inf.relay, log)
	tokens := confirmtoken.New(cfg.JWTSecret, cfg.ConfirmTokenTTL)

	ucs := buildUseCases(repos, subs, inf.gh, tokens, urls, transactor, pub, log)
	svc := connectapi.NewService(ucs.subscribe, ucs.confirm, ucs.unsubscribe, ucs.list, log)

	cleanupUC := cleanup.New(subs, transactor, pub, cfg.ConfirmTokenTTL, log)

	return svc, cleanupUC
}

type useCases struct {
	subscribe   metrics.Metered[subscribe.Input, subscribe.Output]
	confirm     metrics.Metered[confirm.Input, confirm.Output]
	unsubscribe metrics.Metered[unsubscribe.Input, unsubscribe.Output]
	list        metrics.Metered[list.Input, list.Output]
}

func buildUseCases(
	repos *repository.GitHubRepoRepository,
	subs *repository.SubscriptionRepository,
	gh *github.Client,
	tokens *confirmtoken.Tokenizer,
	urls *urlbuilder.URLBuilder,
	tx *db.Transactor,
	pub *eventpublisher.Publisher,
	log *slog.Logger,
) useCases {
	return useCases{
		subscribe: metrics.NewMetered[subscribe.Input, subscribe.Output](
			"subscribe", subscribe.New(repos, subs, gh, tokens, urls, tx, pub, log),
		),
		confirm: metrics.NewMetered[confirm.Input, confirm.Output](
			"confirm", confirm.New(subs, tokens, tx, pub, log),
		),
		unsubscribe: metrics.NewMetered[unsubscribe.Input, unsubscribe.Output](
			"unsubscribe", unsubscribe.New(subs, tx, pub, log),
		),
		list: metrics.NewMetered[list.Input, list.Output](
			"list", list.New(subs),
		),
	}
}
