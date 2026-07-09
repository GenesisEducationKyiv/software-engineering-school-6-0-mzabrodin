package subscription

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github-release-notifier/internal/infrastructure/config"
	connectapi "github-release-notifier/internal/subscription/adapter/connectrpc"
)

func newPublicServer(cfg *config.SubscriptionConfig, svc *connectapi.Service, log *slog.Logger) (*http.Server, error) {
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

type serveDeps struct {
	publicSrv        *http.Server
	internalSrv      *http.Server
	backgroundDone   <-chan struct{}
	cancelBackground context.CancelFunc
	log              *slog.Logger
}

func serve(ctx context.Context, d serveDeps) error {
	serverError := make(chan error, 2)

	go func() {
		d.log.Info("server started", "server", "public", "addr", d.publicSrv.Addr)
		if err := d.publicSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	go func() {
		d.log.Info("server started", "server", "internal-grpc", "addr", d.internalSrv.Addr)
		if err := d.internalSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	d.cancelBackground()
	<-d.backgroundDone

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := d.publicSrv.Shutdown(shutdownCtx); err != nil {
		d.log.Warn("failed to shut down server", "server", "public", "error", err)
	}

	if err := d.internalSrv.Shutdown(shutdownCtx); err != nil {
		d.log.Warn("failed to shut down server", "server", "internal-grpc", "error", err)
	}

	d.log.Info("server stopped")
}
