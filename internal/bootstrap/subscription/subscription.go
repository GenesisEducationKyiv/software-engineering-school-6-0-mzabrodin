package subscription

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/http2"

	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/infrastructure/scheduler"
	"github-release-notifier/internal/infrastructure/tlsconfig"
	"github-release-notifier/internal/infrastructure/urlbuilder"
	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/adapter/mailer"
	"github-release-notifier/internal/notifier/adapter/notifierpublisher"
	"github-release-notifier/internal/scanner/adapter/scannerclient"
	"github-release-notifier/internal/shared/github"
	connectapi "github-release-notifier/internal/subscription/adapter/connectrpc"
	"github-release-notifier/internal/subscription/adapter/repository"
	"github-release-notifier/internal/subscription/usecase/confirm"
	"github-release-notifier/internal/subscription/usecase/list"
	"github-release-notifier/internal/subscription/usecase/scan"
	"github-release-notifier/internal/subscription/usecase/subscribe"
	"github-release-notifier/internal/subscription/usecase/unsubscribe"
)

const shutdownTimeout = bootstrap.ShutdownTimeout

func Run(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	inf, cleanup, err := newInfrastructure(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer cleanup()

	svc, scanUseCase := buildApp(inf, cfg, log)

	publicSrv, err := newPublicServer(cfg, svc, log)
	if err != nil {
		return err
	}

	schedulerDone, cancelScheduler := startScheduler(ctx, scanUseCase, cfg.ScanInterval, log)

	return serve(ctx, serveDeps{
		publicSrv:       publicSrv,
		schedulerDone:   schedulerDone,
		cancelScheduler: cancelScheduler,
		log:             log,
	})
}

type infrastructure struct {
	pool        *pgxpool.Pool
	gh          *github.Client
	notifierPub *notifierpublisher.Publisher
	scannerCli  *scannerclient.Client
}

func newInfrastructure(ctx context.Context, cfg *config.Config, log *slog.Logger) (*infrastructure, func(), error) {
	var closers []func()
	cleanup := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}
	}

	ok := false
	defer func() {
		if !ok {
			cleanup()
		}
	}()

	if err := db.RunMigrations(cfg.DatabaseURL, "file://migrations", log); err != nil {
		return nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL, metrics.NewPgxTracer(), log)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to database: %w", err)
	}
	closers = append(closers, pool.Close)

	redisCache, closeRedis, err := bootstrap.ConnectRedis(ctx, cfg.RedisURL, log)
	if err != nil {
		return nil, nil, err
	}
	closers = append(closers, closeRedis)

	gh := github.NewClient(cfg.GitHubToken, log).WithCache(redisCache, 10*time.Minute)

	brokerConn, closeBroker, err := bootstrap.ConnectBroker(ctx, cfg.NATSURL,
		notifier.StreamEmail, []string{notifier.SubjectConfirmation, notifier.SubjectRelease}, log)
	if err != nil {
		return nil, nil, err
	}
	closers = append(closers, closeBroker)

	notifierPub := notifierpublisher.New(brokerConn, log)
	closers = append(closers, func() {
		if err := notifierPub.Close(); err != nil {
			log.Warn("failed to close notifier publisher", "error", err)
		}
	})

	scannerCli, err := newScannerClient(cfg, log)
	if err != nil {
		return nil, nil, err
	}
	closers = append(closers, func() {
		if err := scannerCli.Close(); err != nil {
			log.Warn("failed to close scanner client", "error", err)
		}
	})

	ok = true

	return &infrastructure{pool: pool, gh: gh, notifierPub: notifierPub, scannerCli: scannerCli}, cleanup, nil
}

func buildApp(inf *infrastructure, cfg *config.Config, log *slog.Logger) (*connectapi.Service, *scan.UseCase) {
	repos := repository.NewGitHubRepoRepository(inf.pool)
	subs := repository.NewSubscriptionRepository(inf.pool)
	urls := urlbuilder.New(cfg.BaseURL)
	releaseNotifier := mailer.NewReleaseNotifier(inf.notifierPub, urls, log)

	ucs := buildUseCases(repos, subs, inf.gh, inf.notifierPub, releaseNotifier, urls, log)
	svc := connectapi.NewService(ucs.subscribe, ucs.confirm, ucs.unsubscribe, ucs.list, log)

	scanUseCase := scan.New(repos, subs, inf.scannerCli, releaseNotifier, log)

	return svc, scanUseCase
}

func startScheduler(
	ctx context.Context,
	uc *scan.UseCase,
	interval time.Duration,
	log *slog.Logger,
) (<-chan struct{}, context.CancelFunc) {
	schedulerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		scheduler.New(scanRunner{uc}, interval, log).Start(schedulerCtx)
		close(done)
	}()

	return done, cancel
}

type scanRunner struct {
	uc *scan.UseCase
}

func (r scanRunner) Run(ctx context.Context) error {
	_, err := r.uc.Execute(ctx, scan.Input{})
	return err
}

func newPublicServer(cfg *config.Config, svc *connectapi.Service, log *slog.Logger) (*http.Server, error) {
	handler, err := NewHandler(svc, cfg.APIKey, log)
	if err != nil {
		return nil, err
	}

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: shutdownTimeout,
		Protocols:         protocols,
	}, nil
}

func newScannerClient(cfg *config.Config, log *slog.Logger) (*scannerclient.Client, error) {
	httpClient, closer, err := mtlsClient(cfg.TLS)
	if err != nil {
		return nil, fmt.Errorf("build scanner tls config: %w", err)
	}

	return scannerclient.New(httpClient, "https://"+cfg.ScannerAddr, closer, log), nil
}

func mtlsClient(tls config.TLSConfig) (*http.Client, func(), error) {
	tlsCfg, err := tlsconfig.ClientTLS(tls.CertFile, tls.KeyFile, tls.CAFile, "")
	if err != nil {
		return nil, nil, fmt.Errorf("build tls config: %w", err)
	}

	transport := &http2.Transport{TLSClientConfig: tlsCfg}

	return &http.Client{Transport: transport}, transport.CloseIdleConnections, nil
}

type serveDeps struct {
	publicSrv       *http.Server
	schedulerDone   <-chan struct{}
	cancelScheduler context.CancelFunc
	log             *slog.Logger
}

func serve(ctx context.Context, d serveDeps) error {
	serverError := make(chan error, 1)

	go func() {
		d.log.Info("server started", "server", "public", "addr", d.publicSrv.Addr)
		if err := d.publicSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	select {
	case err := <-serverError:
		gracefulShutdown(d)
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		d.log.Info("shutting down")
		gracefulShutdown(d)
	}

	return nil
}

func gracefulShutdown(d serveDeps) {
	d.cancelScheduler()
	<-d.schedulerDone

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := d.publicSrv.Shutdown(shutdownCtx); err != nil {
		d.log.Warn("failed to shut down server", "server", "public", "error", err)
	}

	d.log.Info("server stopped")
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
	mail *notifierpublisher.Publisher,
	releaseNotifier *mailer.ReleaseNotifier,
	urls *urlbuilder.URLBuilder,
	log *slog.Logger,
) useCases {
	return useCases{
		subscribe: metrics.NewMetered[subscribe.Input, subscribe.Output](
			"subscribe", subscribe.New(repos, subs, gh, mail, urls, log),
		),
		confirm: metrics.NewMetered[confirm.Input, confirm.Output](
			"confirm", confirm.New(subs, gh, releaseNotifier, log),
		),
		unsubscribe: metrics.NewMetered[unsubscribe.Input, unsubscribe.Output](
			"unsubscribe", unsubscribe.New(subs, log),
		),
		list: metrics.NewMetered[list.Input, list.Output](
			"list", list.New(subs),
		),
	}
}
