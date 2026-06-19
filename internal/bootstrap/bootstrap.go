package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/infrastructure/cache"
	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/shared/events"
)

const ShutdownTimeout = 5 * time.Second

type Config interface {
	SlogLevel() slog.Level
}

func Main[C Config](appName string, load func() (C, error), run func(context.Context, C, *slog.Logger) error) {
	boot := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(boot)

	if err := launch(load, run); err != nil {
		boot.Error(appName+" failed", "error", err)
		os.Exit(1)
	}
}

func launch[C Config](load func() (C, error), run func(context.Context, C, *slog.Logger) error) error {
	if err := godotenv.Load(); err != nil {
		slog.Warn("could not load .env file", "error", err)
	}

	cfg, err := load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := slog.New(
		logging.NewRequestIDHandler(
			logging.NewScanIDHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.SlogLevel()})),
		),
	)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return run(ctx, cfg, log)
}

func ConnectRedis(ctx context.Context, url string, log *slog.Logger) (cache.Cache, func(), error) {
	redisCache, err := cache.NewRedisCache(ctx, url)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to redis: %w", err)
	}
	log.Info("redis connected")

	closer := func() {
		if err := redisCache.Close(); err != nil {
			log.Warn("failed to close redis connection", "error", err)
		}
	}

	return redisCache, closer, nil
}

func ConnectBroker(
	ctx context.Context,
	url, stream string,
	subjects []string,
	log *slog.Logger,
) (*broker.Conn, func(), error) {
	conn, err := broker.Connect(url, log)
	if err != nil {
		return nil, nil, fmt.Errorf("connect broker: %w", err)
	}
	log.Info("broker connected", "url", url)

	closer := func() {
		if err := conn.Close(); err != nil {
			log.Warn("failed to close broker", "error", err)
		}
	}

	if err := conn.EnsureStream(ctx, stream, subjects); err != nil {
		closer()

		return nil, nil, fmt.Errorf("ensure stream: %w", err)
	}

	return conn, closer, nil
}

func EnsureEventStreams(ctx context.Context, conn *broker.Conn) error {
	streams := []struct {
		name     string
		subjects []string
	}{
		{events.StreamSubscriptions, events.SubjectsSubscriptions},
		{events.StreamReleases, events.SubjectsReleases},
		{events.StreamNotifications, events.SubjectsNotifications},
	}

	for _, s := range streams {
		if err := conn.EnsureStream(ctx, s.name, s.subjects); err != nil {
			return fmt.Errorf("ensure stream %q: %w", s.name, err)
		}
	}

	return nil
}

func MetricsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	return mux
}
