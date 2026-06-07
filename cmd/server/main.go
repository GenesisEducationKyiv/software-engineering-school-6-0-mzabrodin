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
	"github-release-notifier/internal/infrastructure/scheduler"
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

	releaseNotifier := mailer.NewReleaseNotifier(mail, urls, log)

	scan := scanner.NewScanner(repos, subs, gh, releaseNotifier, cfg.WorkerCount, log)

	schedulerDone := make(chan struct{})
	go func() {
		scheduler.New(scan, cfg.ScanInterval, log).Start(ctx)
		close(schedulerDone)
	}()

	srv := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: api.NewRouter(buildHandler(repos, subs, gh, mail, urls, log), cfg.APIKey, log),
	}

	serverError := make(chan error, 1)
	go func() {
		log.Info("server started", "port", cfg.HTTPPort)
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

	return gracefulShutdown(srv, schedulerDone, mail, log)
}

func gracefulShutdown(srv *http.Server, schedulerDone <-chan struct{}, mail *mailer.Mailer, log *slog.Logger) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	// HTTP is stopped; wait for the scanner to finish so no producer remains,
	// then drain and close the mailer (bounded by the shutdown deadline).
	<-schedulerDone
	mail.Shutdown(shutdownCtx)

	log.Info("server stopped")
	return nil
}

func buildHandler(
	repos *repository.GitHubRepoRepository,
	subs *repository.SubscriptionRepository,
	gh *github.Client,
	mail *mailer.Mailer,
	urls *urlbuilder.URLBuilder,
	log *slog.Logger,
) *api.Handler {
	subscribeUseCase := subscribe.New(repos, subs, gh, mail, urls, log)
	confirmUseCase := confirm.New(subs, log)
	unsubscribeUseCase := unsubscribe.New(subs, log)
	listUseCase := list.New(subs)

	return api.NewHandler(
		metrics.NewMetered[subscribe.Input, subscribe.Output]("subscribe", subscribeUseCase),
		metrics.NewMetered[confirm.Input, confirm.Output]("confirm", confirmUseCase),
		metrics.NewMetered[unsubscribe.Input, unsubscribe.Output]("unsubscribe", unsubscribeUseCase),
		metrics.NewMetered[list.Input, list.Output]("list", listUseCase),
		log,
	)
}
