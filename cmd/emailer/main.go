package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github-release-notifier/internal/adapter/emailerserver"
	"github-release-notifier/internal/adapter/mailer"
	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/infrastructure/tlsconfig"
)

func main() {
	bootstrap := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(bootstrap)

	if err := run(bootstrap); err != nil {
		bootstrap.Error("emailer failed", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	if err := godotenv.Load(); err != nil {
		log.Warn("could not load .env file", "error", err)
	}

	cfg, err := config.LoadEmailer()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log = slog.New(
		logging.NewRequestIDHandler(
			logging.NewScanIDHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.SlogLevel()})),
		),
	)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

	grpcServer, grpcListener, err := startGRPCServer(ctx, mail, cfg, log)
	if err != nil {
		return err
	}

	metricsSrv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           metricsHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return serve(ctx, grpcServer, grpcListener, metricsSrv, mail, log)
}

func startGRPCServer(
	ctx context.Context,
	mail *mailer.Mailer,
	cfg *config.EmailerConfig,
	log *slog.Logger,
) (*grpc.Server, net.Listener, error) {
	tlsCfg, err := tlsconfig.ServerTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile, cfg.TLS.CAFile)
	if err != nil {
		return nil, nil, fmt.Errorf("build emailer tls config: %w", err)
	}

	handler := emailerserver.NewServer(mail, log)

	grpcServer, err := emailerserver.NewGRPCServer(handler, tlsCfg, log)
	if err != nil {
		return nil, nil, fmt.Errorf("create grpc server: %w", err)
	}

	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return nil, nil, fmt.Errorf("listen grpc: %w", err)
	}

	return grpcServer, listener, nil
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
	grpcServer *grpc.Server,
	grpcListener net.Listener,
	metricsSrv *http.Server,
	mail *mailer.Mailer,
	log *slog.Logger,
) error {
	serverError := make(chan error, 2)

	go func() {
		log.Info("emailer grpc server started", "addr", grpcListener.Addr().String())
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
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

	return gracefulShutdown(grpcServer, metricsSrv, mail, log)
}

func gracefulShutdown(
	grpcServer *grpc.Server,
	metricsSrv *http.Server,
	mail *mailer.Mailer,
	log *slog.Logger,
) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stopGRPCServer(shutdownCtx, grpcServer)

	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Warn("failed to shut down metrics server", "error", err)
	}

	mail.Shutdown(shutdownCtx)

	log.Info("emailer stopped")
	return nil
}

func stopGRPCServer(ctx context.Context, grpcServer *grpc.Server) {
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-ctx.Done():
		grpcServer.Stop()
	}
}
