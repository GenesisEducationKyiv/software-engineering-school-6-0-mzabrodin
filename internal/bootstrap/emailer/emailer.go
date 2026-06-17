package emailer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"buf.build/go/protovalidate"

	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/adapter/mailer"
	"github-release-notifier/internal/notifier/adapter/notifierconsumer"
)

const (
	shutdownTimeout = bootstrap.ShutdownTimeout

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
	defer shutdownMailer(mail)

	validator, err := protovalidate.New()
	if err != nil {
		return fmt.Errorf("create validator: %w", err)
	}

	conn, closeBroker, err := bootstrap.ConnectBroker(ctx, cfg.NATSURL,
		notifier.StreamEmail, []string{notifier.SubjectConfirmation, notifier.SubjectRelease}, log)
	if err != nil {
		return err
	}
	defer closeBroker()

	consumer := notifierconsumer.New(mail, validator, log)

	stop, err := startConsumers(ctx, conn, consumer)
	if err != nil {
		return fmt.Errorf("start consumers: %w", err)
	}
	defer stop()

	metricsSrv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           bootstrap.MetricsHandler(),
		ReadHeaderTimeout: shutdownTimeout,
	}

	return serve(ctx, metricsSrv, log)
}

func shutdownMailer(mail *mailer.Mailer) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	mail.Shutdown(shutdownCtx)
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

func serve(ctx context.Context, metricsSrv *http.Server, log *slog.Logger) error {
	serverError := make(chan error, 1)

	go func() {
		log.Info("server started", "server", "metrics", "addr", metricsSrv.Addr)
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	select {
	case err := <-serverError:
		gracefulShutdown(metricsSrv, log)
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		log.Info("shutting down")
		gracefulShutdown(metricsSrv, log)
	}

	return nil
}

func gracefulShutdown(metricsSrv *http.Server, log *slog.Logger) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Warn("failed to shut down server", "server", "metrics", "error", err)
	}

	log.Info("emailer stopped")
}
