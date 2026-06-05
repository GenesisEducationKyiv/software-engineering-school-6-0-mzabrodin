package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github-release-notifier/internal/api"
	"github-release-notifier/internal/cache"
	"github-release-notifier/internal/config"
	"github-release-notifier/internal/db"
	"github-release-notifier/internal/github"
	"github-release-notifier/internal/mailer"
	"github-release-notifier/internal/repository"
	"github-release-notifier/internal/scanner"
	"github-release-notifier/internal/service"
	"github-release-notifier/internal/urlbuilder"

	"github.com/joho/godotenv"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if err := godotenv.Load(); err != nil {
		slog.Warn("could not load .env file", "error", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := db.RunMigrations(cfg.DatabaseURL, "file://migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	redisCache, err := cache.NewRedisCache(ctx, cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("connect to redis: %w", err)
	}

	defer func() {
		if err := redisCache.Close(); err != nil {
			slog.Warn("failed to close redis connection", "error", err)
		}
	}()

	slog.Info("redis connected")

	gh := github.NewClient(cfg.GitHubToken).WithCache(redisCache, 10*time.Minute)

	mail, err := mailer.NewMailer(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.User, cfg.SMTP.Password, cfg.SMTP.FromEmail)
	if err != nil {
		return fmt.Errorf("create mailer: %w", err)
	}

	repos := repository.NewGitHubRepoRepository(pool)
	subs := repository.NewSubscriptionRepository(pool)
	urls := urlbuilder.New(cfg.BaseURL)

	confirmationNotifier := service.NewConfirmationNotifier(mail)
	releaseNotifier := scanner.NewReleaseNotifier(mail, urls)

	svc := service.NewSubscriptionService(repos, subs, gh, confirmationNotifier, urls)

	scan := scanner.NewScanner(repos, subs, gh, releaseNotifier, cfg.ScanInterval)
	go scan.Start(ctx)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: api.NewRouter(api.NewHandler(svc), cfg.APIKey),
	}

	serverError := make(chan error, 1)
	go func() {
		slog.Info("server started", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	select {
	case err := <-serverError:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		slog.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	svc.Shutdown()

	slog.Info("server stopped")
	return nil
}
