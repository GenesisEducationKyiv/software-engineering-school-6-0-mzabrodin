package scanner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/tlsconfig"
	"github-release-notifier/internal/scanner/adapter/scannerserver"
	scanneruc "github-release-notifier/internal/scanner/usecase/scanner"
	"github-release-notifier/internal/shared/github"
)

const shutdownTimeout = bootstrap.ShutdownTimeout

func Run(ctx context.Context, cfg *config.ScannerConfig, log *slog.Logger) error {
	redisCache, closeRedis, err := bootstrap.ConnectRedis(ctx, cfg.RedisURL, log)
	if err != nil {
		return err
	}
	defer closeRedis()

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
		Handler:           bootstrap.MetricsHandler(),
		ReadHeaderTimeout: shutdownTimeout,
	}

	return serve(ctx, grpcSrv, metricsSrv, log)
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
