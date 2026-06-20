package notifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

type serveDeps struct {
	metricsSrv        *http.Server
	maintenanceDone   <-chan struct{}
	cancelMaintenance context.CancelFunc
	log               *slog.Logger
}

func serve(ctx context.Context, d serveDeps) error {
	serverError := make(chan error, 1)

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
	d.cancelMaintenance()
	<-d.maintenanceDone

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := d.metricsSrv.Shutdown(shutdownCtx); err != nil {
		d.log.Warn("failed to shut down server", "server", "metrics", "error", err)
	}

	d.log.Info("notifier stopped")
}
