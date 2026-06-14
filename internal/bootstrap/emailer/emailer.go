package emailer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"buf.build/go/protovalidate"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/adapter/mailer"
	"github-release-notifier/internal/notifier/adapter/notifierconsumer"
)

const (
	shutdownTimeout = 5 * time.Second

	confirmationConsumer = "emailer-confirmation"
	releaseConsumer      = "emailer-release"
)

func Run(ctx context.Context, cfg *config.EmailerConfig, log *slog.Logger) error {
	mail, err := mailer.NewMailer(
		cfg.SMTP.Host,
		cfg.SMTP.Port,
		cfg.SMTP.User,
		cfg.SMTP.Password,
		cfg.SMTP.FromEmail,
		log,
	)
	if err != nil {
		return fmt.Errorf("create mailer: %w", err)
	}

	validator, err := protovalidate.New()
	if err != nil {
		return fmt.Errorf("create validator: %w", err)
	}

	conn, err := broker.Connect(cfg.NATSURL, log)
	if err != nil {
		return fmt.Errorf("connect broker: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Warn("failed to close broker", "error", err)
		}
	}()
	log.Info("broker connected", "url", cfg.NATSURL)

	if err := conn.EnsureStream(ctx, notifier.StreamEmail,
		[]string{notifier.SubjectConfirmation, notifier.SubjectRelease}); err != nil {
		return fmt.Errorf("ensure stream: %w", err)
	}

	consumer := notifierconsumer.New(mail, validator, log)

	stop, err := startConsumers(ctx, conn, consumer)
	if err != nil {
		return fmt.Errorf("start consumers: %w", err)
	}

	metricsSrv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           metricsHandler(),
		ReadHeaderTimeout: shutdownTimeout,
	}

	return serve(ctx, metricsSrv, stop, mail, log)
}

func startConsumers(ctx context.Context, conn *broker.Conn, consumer *notifierconsumer.Consumer) (func(), error) {
	stopConfirm, err := conn.Consume(ctx, notifier.StreamEmail, confirmationConsumer,
		notifier.SubjectConfirmation, consumer.HandleConfirmation)
	if err != nil {
		return nil, fmt.Errorf("consume confirmations: %w", err)
	}

	stopRelease, err := conn.Consume(ctx, notifier.StreamEmail, releaseConsumer,
		notifier.SubjectRelease, consumer.HandleRelease)
	if err != nil {
		stopConfirm()

		return nil, fmt.Errorf("consume releases: %w", err)
	}

	return func() {
		stopConfirm()
		stopRelease()
	}, nil
}

func metricsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	return mux
}

func serve(
	ctx context.Context,
	metricsSrv *http.Server,
	stopConsumers func(),
	mail *mailer.Mailer,
	log *slog.Logger,
) error {
	serverError := make(chan error, 1)

	go func() {
		log.Info("server started", "server", "metrics", "addr", metricsSrv.Addr)
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	select {
	case err := <-serverError:
		gracefulShutdown(metricsSrv, stopConsumers, mail, log)
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		log.Info("shutting down")
		gracefulShutdown(metricsSrv, stopConsumers, mail, log)
	}

	return nil
}

func gracefulShutdown(
	metricsSrv *http.Server,
	stopConsumers func(),
	mail *mailer.Mailer,
	log *slog.Logger,
) {
	stopConsumers()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Warn("failed to shut down server", "server", "metrics", "error", err)
	}

	mail.Shutdown(shutdownCtx)

	log.Info("emailer stopped")
}
