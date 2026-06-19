package notifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"buf.build/go/protovalidate"
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
	"github-release-notifier/internal/notifier/adapter/notifierconsumer"
	"github-release-notifier/internal/notifier/adapter/repository"
	notifmigrations "github-release-notifier/internal/notifier/migrations"
	"github-release-notifier/internal/notifier/usecase/notifyrelease"
	"github-release-notifier/internal/notifier/usecase/readmodel"
	"github-release-notifier/internal/notifier/usecase/retry"
	"github-release-notifier/internal/notifier/usecase/sendconfirmation"
	"github-release-notifier/internal/shared/events"
)

const shutdownTimeout = bootstrap.ShutdownTimeout

func Run(ctx context.Context, cfg *config.NotifierConfig, log *slog.Logger) error {
	mail, err := mailer.NewMailer(
		cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.User, cfg.SMTP.Password, cfg.SMTP.FromEmail, log,
	)
	if err != nil {
		return fmt.Errorf("create mailer: %w", err)
	}
	// Deferred first so it drains last (LIFO): consumers stop before the mailer
	// drains, so no handler can enqueue onto a closed jobs channel.
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
	// mail is passed twice: it satisfies both releaseSender and confirmationSender.
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

type consumerSpec struct {
	stream  string
	durable string
	subject string
	handler broker.Handler
}

func startConsumers(
	ctx context.Context,
	conn *broker.Conn,
	mail *mailer.Mailer,
	ec *eventconsumer.Consumer,
	log *slog.Logger,
) (func(), error) {
	// The legacy email.* consumer stays until the subscription cutover removes
	// its producer; the new event consumers run alongside it.
	validator, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("create validator: %w", err)
	}

	legacy := notifierconsumer.New(mail, validator, log)

	specs := []consumerSpec{
		{notifier.StreamEmail, "notifier-confirmation", notifier.SubjectConfirmation, legacy.HandleConfirmation},
		{notifier.StreamEmail, "notifier-release", notifier.SubjectRelease, legacy.HandleRelease},
		{
			events.StreamSubscriptions,
			"notifier-subscription-pending",
			events.SubjectSubscriptionPending,
			ec.HandlePending,
		},
		{
			events.StreamSubscriptions,
			"notifier-subscription-confirmed",
			events.SubjectSubscriptionConfirmed,
			ec.HandleConfirmed,
		},
		{
			events.StreamSubscriptions,
			"notifier-subscription-removed",
			events.SubjectSubscriptionRemoved,
			ec.HandleRemoved,
		},
		{events.StreamReleases, "notifier-release-detected", events.SubjectReleaseDetected, ec.HandleReleaseDetected},
	}

	stops := make([]func(), 0, len(specs))
	stopAll := func() {
		for i := len(stops) - 1; i >= 0; i-- {
			stops[i]()
		}
	}

	for _, s := range specs {
		stop, err := conn.Consume(ctx, s.stream, s.durable, s.subject, s.handler)
		if err != nil {
			stopAll()

			return nil, fmt.Errorf("consume %q: %w", s.durable, err)
		}

		stops = append(stops, stop)
	}

	log.Info("consumers started", "count", len(specs))

	return stopAll, nil
}

type maintainer interface {
	Releases(ctx context.Context) error
	Confirmations(ctx context.Context) error
}

type processedPruner interface {
	DeleteProcessedBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

func startMaintenance(
	ctx context.Context,
	retrier maintainer,
	processed processedPruner,
	cfg *config.NotifierConfig,
	log *slog.Logger,
) (<-chan struct{}, context.CancelFunc) {
	maintenanceCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		runMaintenance(maintenanceCtx, retrier, processed, cfg, log)
	}()

	return done, cancel
}

func runMaintenance(
	ctx context.Context,
	retrier maintainer,
	processed processedPruner,
	cfg *config.NotifierConfig,
	log *slog.Logger,
) {
	ticker := time.NewTicker(cfg.RetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			maintenanceTick(ctx, retrier, processed, cfg, log)
		}
	}
}

func maintenanceTick(
	ctx context.Context,
	retrier maintainer,
	processed processedPruner,
	cfg *config.NotifierConfig,
	log *slog.Logger,
) {
	if err := retrier.Releases(ctx); err != nil {
		log.ErrorContext(ctx, "release retry failed", "error", err)
	}

	if err := retrier.Confirmations(ctx); err != nil {
		log.ErrorContext(ctx, "confirmation retry failed", "error", err)
	}

	removed, err := processed.DeleteProcessedBefore(ctx, time.Now().Add(-cfg.ProcessedTTL))
	if err != nil {
		log.ErrorContext(ctx, "prune processed_releases failed", "error", err)

		return
	}

	if removed > 0 {
		log.InfoContext(ctx, "pruned processed_releases", "removed", removed)
	}
}

type serveDeps struct {
	metricsSrv        *http.Server
	maintenanceDone   <-chan struct{}
	cancelMaintenance context.CancelFunc
	log               *slog.Logger
}

func serve(ctx context.Context, d serveDeps) error {
	serverError := make(chan error, 1)

	go func() {
		d.log.Info("server started", "server", "metrics", "addr", d.metricsSrv.Addr)
		if err := d.metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	select {
	case err := <-serverError:
		gracefulShutdown(d)
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		d.log.Info("shutting down")
		gracefulShutdown(d)
	}

	return nil
}

func gracefulShutdown(d serveDeps) {
	d.cancelMaintenance()
	<-d.maintenanceDone

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := d.metricsSrv.Shutdown(shutdownCtx); err != nil {
		d.log.Warn("failed to shut down server", "server", "metrics", "error", err)
	}

	d.log.Info("notifier stopped")
}
