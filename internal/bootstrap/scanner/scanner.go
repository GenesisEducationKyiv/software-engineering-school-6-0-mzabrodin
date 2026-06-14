package scanner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github-release-notifier/internal/infrastructure/cache"
	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/tlsconfig"
	"github-release-notifier/internal/scanner/adapter/scannerserver"
	scanneruc "github-release-notifier/internal/scanner/usecase/scanner"
	"github-release-notifier/internal/shared/github"
)

const shutdownTimeout = 5 * time.Second

func Run(ctx context.Context, cfg *config.ScannerConfig, log *slog.Logger) error {
	redisCache, err := cache.NewRedisCache(ctx, cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("connect to redis: %w", err)
	}
	defer func() {
		if err := redisCache.Close(); err != nil {
			log.Warn("failed to close redis connection", "error", err)
		}
	}()
	log.Info("redis connected")

	gh := github.NewClient(cfg.GitHubToken, log).WithCache(redisCache, 10*time.Minute)
	scanner := scanneruc.New(gh, cfg.WorkerCount, log)

	tlsCfg, err := tlsconfig.ServerTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile, cfg.TLS.CAFile)
	if err != nil {
		return fmt.Errorf("build scanner tls config: %w", err)
	}

	handler, err := scannerserver.NewHandler(scannerserver.NewServer(scanner, log), log)
	if err != nil {
		return fmt.Errorf("create scanner handler: %w", err)
	}

	protocols := new(http.Protocols)
	protocols.SetHTTP2(true)

	grpcSrv := &http.Server{
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

	return serve(ctx, grpcSrv, metricsSrv, log)
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

func serve(ctx context.Context, grpcSrv, metricsSrv *http.Server, log *slog.Logger) error {
	serverError := make(chan error, 2)

	go func() {
		log.Info("server started", "server", "grpc", "addr", grpcSrv.Addr)
		if err := grpcSrv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
		gracefulShutdown(grpcSrv, metricsSrv, log)
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		log.Info("shutting down")
		gracefulShutdown(grpcSrv, metricsSrv, log)
	}

	return nil
}

func gracefulShutdown(grpcSrv, metricsSrv *http.Server, log *slog.Logger) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := grpcSrv.Shutdown(shutdownCtx); err != nil {
		log.Warn("failed to shut down server", "server", "grpc", "error", err)
	}

	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Warn("failed to shut down server", "server", "metrics", "error", err)
	}

	log.Info("scanner stopped")
}
