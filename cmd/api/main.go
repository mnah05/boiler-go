package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boiler-go/internal/config"
	"boiler-go/internal/db"
	"boiler-go/internal/handler"
	"boiler-go/pkg/logger"

	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()
	logg := logger.New()
	ctx := context.Background()

	// Initialize database pool
	if err := db.Open(cfg); err != nil {
		logg.Fatal().Err(err).Msg("failed to initialize database")
	}
	logg.Info().Msg("database connected")

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		logg.Fatal().Err(err).Msg("redis connection failed")
	}
	logg.Info().Msg("redis connected")

	router := handler.NewRouter(logg, cfg, db.Get(), rdb)

	server := &http.Server{
		Addr:           ":" + cfg.AppPort,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	serverErrors := make(chan error, 1)

	go func() {
		logg.Info().
			Str("port", cfg.AppPort).
			Msg("server starting")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("server failed to start: %w", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logg.Fatal().Err(err).Msg("server startup failed")
	case sig := <-stop:
		logg.Info().Str("signal", sig.String()).Msg("shutdown signal received")
	}

	logg.Info().Msg("shutting down server...")

	// Graceful shutdown: stop accepting new connections, then wait for in-flight requests
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.APIShutdownTimeout)
	defer cancel()

	// Shutdown gracefully shuts down the server without interrupting any active connections
	// It first closes all open listeners, then closes all idle connections,
	// and then waits indefinitely for connections to return to idle and then shut down
	if err := server.Shutdown(shutdownCtx); err != nil {
		logg.Error().Err(err).Msg("server shutdown failed")
	} else {
		logg.Info().Msg("server shutdown completed gracefully")
	}

	// close resources in reverse order of initialization
	if err := rdb.Close(); err != nil {
		logg.Error().Err(err).Msg("redis close failed")
	}
	db.Close()

	logg.Info().Msg("server stopped cleanly")
}
