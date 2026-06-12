package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/net/http2"

	"github-release-notifier/internal/infrastructure/cache"
	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/infrastructure/urlbuilder"
	"github-release-notifier/internal/notifier/adapter/emailerclient"
	"github-release-notifier/internal/notifier/adapter/mailer"
	"github-release-notifier/internal/notifier/tlsconfig"
	"github-release-notifier/internal/scanner/scheduler"
	"github-release-notifier/internal/scanner/usecase/scanner"
	"github-release-notifier/internal/shared/github"
	connectapi "github-release-notifier/internal/subscription/adapter/connectrpc"
	"github-release-notifier/internal/subscription/adapter/repository"
	"github-release-notifier/internal/subscription/usecase/confirm"
	"github-release-notifier/internal/subscription/usecase/list"
	"github-release-notifier/internal/subscription/usecase/subscribe"
	"github-release-notifier/internal/subscription/usecase/unsubscribe"
)

const shutdownTimeout = 5 * time.Second

func Run(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
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

	emailerCli, err := newEmailerClient(cfg, log)
	if err != nil {
		return err
	}

	repos := repository.NewGitHubRepoRepository(pool)
	subs := repository.NewSubscriptionRepository(pool)
	urls := urlbuilder.New(cfg.BaseURL)

	releaseNotifier := mailer.NewReleaseNotifier(emailerCli, urls, log)
	scan := scanner.NewScanner(repos, subs, gh, releaseNotifier, cfg.WorkerCount, log)

	schedulerDone := make(chan struct{})
	go func() {
		scheduler.New(scan, cfg.ScanInterval, log).Start(ctx)
		close(schedulerDone)
	}()

	ucs := buildUseCases(repos, subs, gh, emailerCli, urls, log)
	svc := connectapi.NewService(ucs.subscribe, ucs.confirm, ucs.unsubscribe, ucs.list, log)

	handler, err := NewHandler(svc, cfg.APIKey, log)
	if err != nil {
		return err
	}

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: shutdownTimeout,
		Protocols:         protocols,
	}

	return serve(ctx, srv, schedulerDone, emailerCli, log)
}

func newEmailerClient(cfg *config.Config, log *slog.Logger) (*emailerclient.Client, error) {
	tlsCfg, err := tlsconfig.ClientTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile, cfg.TLS.CAFile, "")
	if err != nil {
		return nil, fmt.Errorf("build emailer tls config: %w", err)
	}

	transport := &http2.Transport{TLSClientConfig: tlsCfg}
	httpClient := &http.Client{Transport: transport}

	return emailerclient.New(httpClient, "https://"+cfg.EmailerAddr, transport.CloseIdleConnections, log), nil
}

func serve(
	ctx context.Context,
	srv *http.Server,
	schedulerDone <-chan struct{},
	emailerCli *emailerclient.Client,
	log *slog.Logger,
) error {
	serverError := make(chan error, 1)

	go func() {
		log.Info("server started", "server", "public", "addr", srv.Addr)
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

	return gracefulShutdown(srv, schedulerDone, emailerCli, log)
}

func gracefulShutdown(
	srv *http.Server,
	schedulerDone <-chan struct{},
	emailerCli *emailerclient.Client,
	log *slog.Logger,
) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	<-schedulerDone

	if err := emailerCli.Close(); err != nil {
		log.Warn("failed to close emailer client", "error", err)
	}

	log.Info("server stopped")

	return nil
}

type useCases struct {
	subscribe   metrics.Metered[subscribe.Input, subscribe.Output]
	confirm     metrics.Metered[confirm.Input, confirm.Output]
	unsubscribe metrics.Metered[unsubscribe.Input, unsubscribe.Output]
	list        metrics.Metered[list.Input, list.Output]
}

func buildUseCases(
	repos *repository.GitHubRepoRepository,
	subs *repository.SubscriptionRepository,
	gh *github.Client,
	mail *emailerclient.Client,
	urls *urlbuilder.URLBuilder,
	log *slog.Logger,
) useCases {
	return useCases{
		subscribe: metrics.NewMetered[subscribe.Input, subscribe.Output](
			"subscribe", subscribe.New(repos, subs, gh, mail, urls, log),
		),
		confirm: metrics.NewMetered[confirm.Input, confirm.Output](
			"confirm", confirm.New(subs, log),
		),
		unsubscribe: metrics.NewMetered[unsubscribe.Input, unsubscribe.Output](
			"unsubscribe", unsubscribe.New(subs, log),
		),
		list: metrics.NewMetered[list.Input, list.Output](
			"list", list.New(subs),
		),
	}
}
