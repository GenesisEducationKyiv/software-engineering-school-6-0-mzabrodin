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
	"google.golang.org/grpc/credentials"

	"github-release-notifier/internal/infrastructure/cache"
	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/infrastructure/urlbuilder"
	"github-release-notifier/internal/notifier/adapter/emailerclient"
	"github-release-notifier/internal/notifier/adapter/mailer"
	"github-release-notifier/internal/notifier/tlsconfig"
	"github-release-notifier/internal/scanner/scheduler"
	"github-release-notifier/internal/scanner/usecase/scanner"
	"github-release-notifier/internal/shared/github"
	grpcapi "github-release-notifier/internal/subscription/adapter/grpc"
	api "github-release-notifier/internal/subscription/adapter/http"
	"github-release-notifier/internal/subscription/adapter/repository"
	"github-release-notifier/internal/subscription/usecase/confirm"
	"github-release-notifier/internal/subscription/usecase/list"
	"github-release-notifier/internal/subscription/usecase/subscribe"
	"github-release-notifier/internal/subscription/usecase/unsubscribe"

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

	emailerConn, err := dialEmailer(cfg)
	if err != nil {
		return err
	}
	emailerCli := emailerclient.New(emailerConn, log)

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

	srv := &http.Server{
		Addr: ":" + cfg.HTTPPort,
		Handler: api.NewRouter(
			api.NewHandler(ucs.subscribe, ucs.confirm, ucs.unsubscribe, ucs.list, log),
			cfg.APIKey,
			log,
		),
	}

	grpcHandler := grpcapi.NewServer(ucs.subscribe, ucs.confirm, ucs.unsubscribe, ucs.list, log)

	grpcServer, grpcListener, err := startGRPCServer(ctx, grpcHandler, cfg.APIKey, cfg.GRPCPort, log)
	if err != nil {
		return err
	}

	return serve(ctx, srv, grpcServer, grpcListener, schedulerDone, emailerCli, log)
}

func dialEmailer(cfg *config.Config) (*grpc.ClientConn, error) {
	tlsCfg, err := tlsconfig.ClientTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile, cfg.TLS.CAFile, cfg.EmailerServerName)
	if err != nil {
		return nil, fmt.Errorf("build emailer tls config: %w", err)
	}

	conn, err := grpc.NewClient(cfg.EmailerAddr, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	if err != nil {
		return nil, fmt.Errorf("dial emailer: %w", err)
	}

	return conn, nil
}

func startGRPCServer(
	ctx context.Context,
	handler *grpcapi.Server,
	apiKey, port string,
	log *slog.Logger,
) (*grpc.Server, net.Listener, error) {
	grpcServer, err := grpcapi.NewGRPCServer(handler, apiKey, log)
	if err != nil {
		return nil, nil, fmt.Errorf("create grpc server: %w", err)
	}

	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", ":"+port)
	if err != nil {
		return nil, nil, fmt.Errorf("listen grpc: %w", err)
	}

	return grpcServer, listener, nil
}

func serve(
	ctx context.Context,
	srv *http.Server,
	grpcServer *grpc.Server,
	grpcListener net.Listener,
	schedulerDone <-chan struct{},
	emailerCli *emailerclient.Client,
	log *slog.Logger,
) error {
	serverError := make(chan error, 2)

	go func() {
		log.Info("http server started", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	go func() {
		log.Info("grpc server started", "addr", grpcListener.Addr().String())
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			serverError <- err
		}
	}()

	select {
	case err := <-serverError:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		log.Info("shutting down")
	}

	return gracefulShutdown(srv, grpcServer, schedulerDone, emailerCli, log)
}

func gracefulShutdown(
	srv *http.Server,
	grpcServer *grpc.Server,
	schedulerDone <-chan struct{},
	emailerCli *emailerclient.Client,
	log *slog.Logger,
) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	stopGRPCServer(shutdownCtx, grpcServer)

	<-schedulerDone

	if err := emailerCli.Close(); err != nil {
		log.Warn("failed to close emailer client", "error", err)
	}

	log.Info("server stopped")
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
