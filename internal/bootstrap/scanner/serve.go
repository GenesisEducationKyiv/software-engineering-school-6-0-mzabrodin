package scanner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

type serveDeps struct {
	grpcSrv         *http.Server
	metricsSrv      *http.Server
	schedulerDone   <-chan struct{}
	cancelScheduler context.CancelFunc
	log             *slog.Logger
}

func serve(ctx context.Context, d serveDeps) error {
	serverError := make(chan error, 2)

	go func() {
		d.log.Info("server started", "server", "grpc", "addr", d.grpcSrv.Addr)
		if err := d.grpcSrv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	go func() {
		d.log.Info("server started", "server", "metrics", "addr", d.metricsSrv.Addr)
		if err := d.metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

	if err := d.grpcSrv.Shutdown(shutdownCtx); err != nil {
		d.log.Warn("failed to shut down server", "server", "grpc", "error", err)
	}

	if err := d.metricsSrv.Shutdown(shutdownCtx); err != nil {
		d.log.Warn("failed to shut down server", "server", "metrics", "error", err)
	}

	d.log.Info("scanner stopped")
}
