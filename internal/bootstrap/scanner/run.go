package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/scanner/adapter/eventconsumer"
	"github-release-notifier/internal/scanner/adapter/eventpublisher"
	"github-release-notifier/internal/scanner/adapter/repository"
	scanmigrations "github-release-notifier/internal/scanner/migrations"
	"github-release-notifier/internal/scanner/usecase/advancetag"
	scanneruc "github-release-notifier/internal/scanner/usecase/scanner"
	"github-release-notifier/internal/scanner/usecase/watch"
	"github-release-notifier/internal/scanner/usecase/watchlist"
	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/shared/github"
)

const shutdownTimeout = bootstrap.ShutdownTimeout

func Run(ctx context.Context, cfg *config.ScannerConfig, log *slog.Logger) error {
	redisCache, closeRedis, err := bootstrap.ConnectRedis(ctx, cfg.RedisURL, log)
	if err != nil {
		return err
	}
	defer closeRedis()

	if err := db.RunMigrationsFS(cfg.DatabaseURL, scanmigrations.FS, log); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL, metrics.NewPgxTracer(), log)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	conn, closeBroker, err := bootstrap.ConnectBroker(
		ctx, cfg.NATSURL, events.StreamReleases, events.SubjectsReleases, log,
	)
	if err != nil {
		return err
	}
	defer closeBroker()

	if err := bootstrap.EnsureEventStreams(ctx, conn); err != nil {
		return err
	}

	gh := github.NewClient(cfg.GitHubToken, log).WithCache(redisCache, 10*time.Minute)
	scanner := scanneruc.New(gh, cfg.WorkerCount, log)

	comps := buildComponents(pool, conn, scanner, log)

	metricsSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           bootstrap.MetricsHandler(),
		ReadHeaderTimeout: shutdownTimeout,
	}

	stop, err := startConsumers(ctx, conn, comps.eventConsumer, log)
	if err != nil {
		return fmt.Errorf("start consumers: %w", err)
	}
	defer stop()

	schedulerDone, cancelScheduler := startScheduler(ctx, comps.watch, cfg, log)

	return serve(ctx, serveDeps{
		metricsSrv:      metricsSrv,
		schedulerDone:   schedulerDone,
		cancelScheduler: cancelScheduler,
		log:             log,
	})
}

type components struct {
	watch         *watch.UseCase
	eventConsumer *eventconsumer.Consumer
}

func buildComponents(pool *pgxpool.Pool, conn *broker.Conn, scanner *scanneruc.Scanner, log *slog.Logger) components {
	repo := repository.NewWatchedRepoRepository(pool)
	pub := eventpublisher.New(conn, log)

	projector := watchlist.New(repo, log)
	advance := advancetag.New(repo, log)
	watchUC := watch.New(repo, scanner, pub, log)

	return components{
		watch:         watchUC,
		eventConsumer: eventconsumer.New(projector, advance, log),
	}
}
