// Package graceful provides shared graceful shutdown utilities for server components.
package graceful

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
)

// ShutdownFunc is a function that performs component-specific shutdown.
type ShutdownFunc func(ctx context.Context) error

// ServerShutdown gracefully shuts down an HTTP server.
func ServerShutdown(server *http.Server, timeout time.Duration, logg zerolog.Logger) ShutdownFunc {
	return func(ctx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}
		return nil
	}
}

// WorkerShutdown gracefully shuts down an Asynq server.
func WorkerShutdown(srv *asynq.Server, timeout time.Duration, logg zerolog.Logger) func() {
	return func() {
		// First stop accepting new tasks
		srv.Stop()
		logg.Info().Msg("worker stopped accepting new tasks")

		// Shutdown with timeout enforcement for in-flight tasks
		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Run srv.Shutdown() in a goroutine since it blocks until all in-flight tasks complete
		done := make(chan struct{})
		go func() {
			srv.Shutdown()
			close(done)
		}()

		select {
		case <-done:
			logg.Info().Msg("worker shutdown completed gracefully")
		case <-shutdownCtx.Done():
			logg.Warn().Msg("worker shutdown timed out, forcing exit")
		}
	}
}

// WaitForSignal returns a channel that receives interrupt signals.
// The caller should use this in a select statement.
func WaitForSignal(logg zerolog.Logger) <-chan os.Signal {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	
	// Wrap the signal channel to log when a signal is received
	wrapped := make(chan os.Signal, 1)
	go func() {
		sig := <-stop
		logg.Info().Str("signal", sig.String()).Msg("shutdown signal received")
		wrapped <- sig
	}()
	
	return wrapped
}

// ShutdownConfig holds shutdown configuration for a component.
type ShutdownConfig struct {
	Timeout   time.Duration
	Component string
}
