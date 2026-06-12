package emailer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/notifier/adapter/emailerserver"
	"github-release-notifier/internal/notifier/adapter/mailer"
	"github-release-notifier/internal/notifier/tlsconfig"
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

	apiSrv, listener, err := startServer(ctx, mail, cfg, log)
	if err != nil {
		return err
	}

	metricsSrv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           metricsHandler(),
		ReadHeaderTimeout: shutdownTimeout,
	}

	return serve(ctx, apiSrv, listener, metricsSrv, mail, log)
}

func startServer(
	ctx context.Context,
	mail *mailer.Mailer,
	cfg *config.EmailerConfig,
	log *slog.Logger,
) (*http.Server, net.Listener, error) {
	tlsCfg, err := tlsconfig.ServerTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile, cfg.TLS.CAFile)
	if err != nil {
		return nil, nil, fmt.Errorf("build emailer tls config: %w", err)
	}

	handler, err := emailerserver.NewHandler(emailerserver.NewServer(mail, log), log)
	if err != nil {
		return nil, nil, fmt.Errorf("create emailer handler: %w", err)
	}

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)

	srv := &http.Server{
		Handler:           handler,
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: shutdownTimeout,
		Protocols:         protocols,
	}

	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return nil, nil, fmt.Errorf("listen grpc: %w", err)
	}

	return srv, listener, nil
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
	listener net.Listener,
	metricsSrv *http.Server,
	mail *mailer.Mailer,
	log *slog.Logger,
) error {
	serverError := make(chan error, 2)

	go func() {
		log.Info("emailer grpc server started", "addr", listener.Addr().String())
		if err := apiSrv.ServeTLS(listener, "", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	go func() {
		log.Info("emailer metrics server started", "addr", metricsSrv.Addr)
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
		log.Warn("failed to shut down emailer server", "error", err)
	}

	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Warn("failed to shut down metrics server", "error", err)
	}

	mail.Shutdown(shutdownCtx)

	log.Info("emailer stopped")

	return nil
}
