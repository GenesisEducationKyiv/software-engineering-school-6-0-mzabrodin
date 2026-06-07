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

	"github-release-notifier/internal/adapter/cache"
	"github-release-notifier/internal/adapter/github"
	api "github-release-notifier/internal/adapter/http"
	"github-release-notifier/internal/adapter/mailer"
	"github-release-notifier/internal/adapter/repository"
	"github-release-notifier/internal/adapter/urlbuilder"
	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/usecase/confirm"
	"github-release-notifier/internal/usecase/list"
	"github-release-notifier/internal/usecase/scanner"
	"github-release-notifier/internal/usecase/subscribe"
	"github-release-notifier/internal/usecase/unsubscribe"

	"github.com/joho/godotenv"
)

func main() {
	bootstrap := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(bootstrap)

	if err := run(bootstrap); err != nil {
		bootstrap.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	if err := godotenv.Load(); err != nil {
		log.Warn("could not load .env file", "error", err)
	}

	cfg, err := config.Load()
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

	if err := db.RunMigrations(cfg.DatabaseURL, "file://migrations", log); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL, metrics.NewPgxTracer(), log)
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
			log.Warn("failed to close redis connection", "error", err)
		}
	}()

	log.Info("redis connected")

	gh := github.NewClient(cfg.GitHubToken, log).WithCache(redisCache, 10*time.Minute)

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

	repos := repository.NewGitHubRepoRepository(pool)
	subs := repository.NewSubscriptionRepository(pool)
	urls := urlbuilder.New(cfg.BaseURL)

	confirmationNotifier := mailer.NewConfirmationNotifier(mail, log)
	releaseNotifier := scanner.NewReleaseNotifier(mail, urls)

	scan := scanner.NewScanner(repos, subs, gh, releaseNotifier, cfg.ScanInterval, log)
	go scan.Start(ctx)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: api.NewRouter(buildHandler(repos, subs, gh, confirmationNotifier, urls, log), cfg.APIKey, log),
	}

	serverError := make(chan error, 1)
	go func() {
		log.Info("server started", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	select {
	case err := <-serverError:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	confirmationNotifier.Shutdown()

	log.Info("server stopped")
	return nil
}

func buildHandler(
	repos *repository.GitHubRepoRepository,
	subs *repository.SubscriptionRepository,
	gh *github.Client,
	confirmationNotifier *mailer.ConfirmationNotifier,
	urls *urlbuilder.URLBuilder,
	log *slog.Logger,
) *api.Handler {
	subscribeUseCase := subscribe.New(repos, subs, gh, confirmationNotifier, urls, log)
	confirmUseCase := confirm.New(subs, log)
	unsubscribeUseCase := unsubscribe.New(subs, log)
	listUseCase := list.New(subs)

	return api.NewHandler(subscribeUseCase, confirmUseCase, unsubscribeUseCase, listUseCase, log)
}
