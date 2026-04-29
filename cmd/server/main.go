package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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

	"github.com/joho/godotenv"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := godotenv.Load(); err != nil {
		slog.Warn("could not load .env file", "error", err)
	}

	cfg := config.Load()

	if err := db.RunMigrations(cfg.MigrateDSN); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	pool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	repos := repository.NewRepoRepository(pool)
	subs := repository.NewSubscriptionRepository(pool)

	redisCache, err := cache.NewRedisCache(context.Background(), cfg.RedisURL)
	if err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}

	defer func() {
		if err := redisCache.Close(); err != nil {
			slog.Warn("failed to close redis connection", "error", err)
		}
	}()

	slog.Info("redis connected")

	gh := github.NewClient(cfg.GitHubToken).WithCache(redisCache, 10*time.Minute)

	smtpPort, err := strconv.Atoi(cfg.SMTP.Port)
	if err != nil {
		slog.Error("invalid SMTP port", "error", err)
		os.Exit(1)
	}

	mail := mailer.NewMailer(cfg.SMTP.Host, smtpPort, cfg.SMTP.User, cfg.SMTP.Password, cfg.SMTP.FromEmail)

	svc := service.NewSubscriptionService(repos, subs, gh, mail, cfg.BaseURL)

	scannerCtx, cancelScanner := context.WithCancel(context.Background())
	defer cancelScanner()
	scan := scanner.NewScanner(repos, subs, gh, mail, cfg.ScanInterval, cfg.BaseURL)
	go scan.Start(scannerCtx)

	handler := api.NewHandler(svc)
	router := api.NewRouter(handler, cfg.APIKey)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		slog.Info("server started", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	cancelScanner()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server stopped")
}
