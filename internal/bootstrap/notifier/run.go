package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/infrastructure/urlbuilder"
	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/adapter/eventconsumer"
	"github-release-notifier/internal/notifier/adapter/eventpublisher"
	"github-release-notifier/internal/notifier/adapter/mailer"
	"github-release-notifier/internal/notifier/adapter/repository"
	notifmigrations "github-release-notifier/internal/notifier/migrations"
	"github-release-notifier/internal/notifier/usecase/notifyrelease"
	"github-release-notifier/internal/notifier/usecase/readmodel"
	"github-release-notifier/internal/notifier/usecase/retry"
	"github-release-notifier/internal/notifier/usecase/sendconfirmation"
)

const shutdownTimeout = bootstrap.ShutdownTimeout

func Run(ctx context.Context, cfg *config.NotifierConfig, log *slog.Logger) error {
	mail, err := mailer.NewMailer(
		cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.User, cfg.SMTP.Password, cfg.SMTP.FromEmail, log,
	)
	if err != nil {
		return fmt.Errorf("create mailer: %w", err)
	}
	defer shutdownMailer(mail)

	if err := db.RunMigrationsFS(cfg.DatabaseURL, notifmigrations.FS, log); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL, metrics.NewPgxTracer(), log)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	conn, closeBroker, err := bootstrap.ConnectBroker(ctx, cfg.NATSURL,
		notifier.StreamEmail, []string{notifier.SubjectConfirmation, notifier.SubjectRelease}, log)
	if err != nil {
		return err
	}
	defer closeBroker()

	if err := bootstrap.EnsureEventStreams(ctx, conn); err != nil {
		return err
	}

	comps := buildComponents(pool, conn, mail, cfg, log)

	stop, err := startConsumers(ctx, conn, mail, comps.eventConsumer, log)
	if err != nil {
		return fmt.Errorf("start consumers: %w", err)
	}
	defer stop()

	maintenanceDone, cancelMaintenance := startMaintenance(ctx, comps.retrier, comps.processed, cfg, log)

	metricsSrv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           bootstrap.MetricsHandler(),
		ReadHeaderTimeout: shutdownTimeout,
	}

	return serve(ctx, serveDeps{
		metricsSrv:        metricsSrv,
		maintenanceDone:   maintenanceDone,
		cancelMaintenance: cancelMaintenance,
		log:               log,
	})
}

type components struct {
	eventConsumer *eventconsumer.Consumer
	retrier       *retry.Retrier
	processed     *repository.ProcessedReleasesRepository
}

func buildComponents(
	pool *pgxpool.Pool,
	conn *broker.Conn,
	mail *mailer.Mailer,
	cfg *config.NotifierConfig,
	log *slog.Logger,
) components {
	readRepo := repository.NewSubscriptionsReadRepository(pool)
	failedNotif := repository.NewFailedNotificationsRepository(pool)
	failedConf := repository.NewFailedConfirmationsRepository(pool)
	processed := repository.NewProcessedReleasesRepository(pool)

	pub := eventpublisher.New(conn, log)
	urls := urlbuilder.New(cfg.BaseURL)

	confirmUC := sendconfirmation.New(mail, failedConf, pub, log)
	projector := readmodel.New(readRepo)
	releaseUC := notifyrelease.New(readRepo, processed, failedNotif, mail, urls, pub, log)
	retrier := retry.New(failedNotif, failedConf, readRepo, mail, mail, urls, pub, retry.Config{
		MaxRetries:      cfg.MaxRetries,
		ConfirmationTTL: cfg.ConfirmationTTL,
	}, log)

	return components{
		eventConsumer: eventconsumer.New(confirmUC, projector, releaseUC, log),
		retrier:       retrier,
		processed:     processed,
	}
}

func shutdownMailer(mail *mailer.Mailer) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	mail.Shutdown(shutdownCtx)
}
