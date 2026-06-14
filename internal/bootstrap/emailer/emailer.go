package emailer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/tlsconfig"
	"github-release-notifier/internal/notifier/adapter/mailer"
	"github-release-notifier/internal/notifier/adapter/notifierserver"
)

const shutdownTimeout = 5 * time.Second

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

	tlsCfg, err := tlsconfig.ServerTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile, cfg.TLS.CAFile)
	if err != nil {
		return fmt.Errorf("build emailer tls config: %w", err)
	}

	handler, err := notifierserver.NewHandler(notifierserver.NewServer(mail, log), log)
	if err != nil {
		return fmt.Errorf("create emailer handler: %w", err)
	}

	protocols := new(http.Protocols)
	protocols.SetHTTP2(true)

	apiSrv := &http.Server{
		Addr:              ":" + cfg.GRPCPort,
		Handler:           handler,
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: shutdownTimeout,
		Protocols:         protocols,
	}

	metricsSrv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           metricsHandler(),
		ReadHeaderTimeout: shutdownTimeout,
	}

	return serve(ctx, apiSrv, metricsSrv, mail, log)
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
	apiSrv *http.Server,
	metricsSrv *http.Server,
	mail *mailer.Mailer,
	log *slog.Logger,
) error {
	serverError := make(chan error, 2)

	go func() {
		log.Info("server started", "server", "grpc", "addr", apiSrv.Addr)
		if err := apiSrv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	go func() {
		log.Info("server started", "server", "metrics", "addr", metricsSrv.Addr)
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	select {
	case err := <-serverError:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		log.Info("shutting down")
	}

	return gracefulShutdown(apiSrv, metricsSrv, mail, log)
}

func gracefulShutdown(
	apiSrv *http.Server,
	metricsSrv *http.Server,
	mail *mailer.Mailer,
	log *slog.Logger,
) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := apiSrv.Shutdown(shutdownCtx); err != nil {
		log.Warn("failed to shut down server", "server", "grpc", "error", err)
	}

	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Warn("failed to shut down server", "server", "metrics", "error", err)
	}

	mail.Shutdown(shutdownCtx)

	log.Info("server stopped")

	return nil
}
