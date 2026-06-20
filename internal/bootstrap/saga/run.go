package saga

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/infrastructure/outbox"
	"github-release-notifier/internal/saga/adapter/eventconsumer"
	"github-release-notifier/internal/saga/adapter/eventpublisher"
	"github-release-notifier/internal/saga/adapter/repository"
	sagamigrations "github-release-notifier/internal/saga/migrations"
	"github-release-notifier/internal/saga/usecase/coordinator"
	"github-release-notifier/internal/shared/events"
)

const (
	shutdownTimeout = bootstrap.ShutdownTimeout
	relayInterval   = 5 * time.Second
	relayBatchSize  = 100
)

func Run(ctx context.Context, cfg *config.SagaConfig, log *slog.Logger) error {
	if err := db.RunMigrationsFS(cfg.DatabaseURL, sagamigrations.FS, log); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	if err := db.RunMigrationsFS(
		cfg.DatabaseURL, outbox.Migrations, log, db.WithMigrationsTable("outbox_schema_migrations"),
	); err != nil {
		return fmt.Errorf("run outbox migrations: %w", err)
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL, metrics.NewPgxTracer(), log)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	conn, closeBroker, err := bootstrap.ConnectBroker(ctx, cfg.NATSURL,
		events.StreamSagas, events.SubjectsSagas, log)
	if err != nil {
		return err
	}
	defer closeBroker()

	if err := bootstrap.EnsureEventStreams(ctx, conn); err != nil {
		return err
	}

	repo := repository.NewSagaRepository(pool)
	transactor := db.NewTransactor(pool)
	relay := outbox.NewRelay(pool, conn, relayInterval, relayBatchSize, log)
	pub := eventpublisher.New(relay, log)
	coord := coordinator.New(repo, pub, transactor, log)
	ec := eventconsumer.New(coord, log)

	stop, err := startConsumers(ctx, conn, ec, log)
	if err != nil {
		return fmt.Errorf("start consumers: %w", err)
	}
	defer stop()

	go relay.Run(ctx)

	metricsSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           bootstrap.MetricsHandler(),
		ReadHeaderTimeout: shutdownTimeout,
	}

	return serve(ctx, serveDeps{metricsSrv: metricsSrv, log: log})
}
