package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// serveHTTP drains active requests on cancellation or listener failure before
// returning to the caller, which owns the database and other dependencies.
func serveHTTP(ctx context.Context, srv *http.Server, listener net.Listener, drainDelay, shutdownTimeout time.Duration) error {
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- srv.Serve(listener) }()

	var serveErr error
	serveFinished := false
	select {
	case <-ctx.Done():
		// Readiness is already withdrawn. Keep serving while routing updates propagate.
		timer := time.NewTimer(drainDelay)
		select {
		case <-timer.C:
		case serveErr = <-serveErrors:
			serveFinished = true
		}
		timer.Stop()
	case serveErr = <-serveErrors:
		serveFinished = true
	}

	// The request to stop must not cancel the graceful shutdown deadline.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	shutdownErr := srv.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		shutdownErr = fmt.Errorf("HTTP shutdown: %w", shutdownErr)
		if err := srv.Close(); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("HTTP forced close: %w", err))
		}
	}
	if !serveFinished {
		serveErr = <-serveErrors
	}
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	} else if serveErr != nil {
		serveErr = fmt.Errorf("HTTP serve: %w", serveErr)
	}
	return errors.Join(serveErr, shutdownErr)
}
