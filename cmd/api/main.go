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
	"boiler-go/internal/scheduler"
	"boiler-go/pkg/logger"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Load config first with basic logger
	cfg := config.Load(logger.New())

	// Create logger based on configuration
	logg, logCleanup := logger.NewLogger(cfg, "logs/api.log")
	if logCleanup != nil {
		defer logCleanup()
	}
	ctx := context.Background()

	// Initialize database pool with timeout context
	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()
	if err := db.Open(dbCtx, cfg); err != nil {
		logg.Fatal().Err(err).Msg("failed to initialize database")
	}
	logg.Info().Msg("database connected")

	pool := db.Get()
	if pool == nil {
		logg.Fatal().Msg("database pool is nil")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		logg.Fatal().Err(err).Msg("redis connection failed")
	}
	logg.Info().Msg("redis connected")

	// Initialize scheduler client for worker task enqueueing
	schedulerClient := scheduler.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	logg.Info().Msg("scheduler client initialized")

	router := handler.NewRouter(logg, cfg, pool, rdb, schedulerClient)

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

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logg.Fatal().Err(err).Msg("server startup failed")
	case sig := <-sigChan:
		logg.Info().Str("signal", sig.String()).Msg("shutdown signal received")
	}

	logg.Info().Msg("shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.APIShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logg.Error().Err(err).Msg("server shutdown failed")
	} else {
		logg.Info().Msg("server shutdown completed gracefully")
	}

	// Close resources in reverse order of initialization:
	// 1. Scheduler client (must close before Redis since it uses Redis connection)
	if err := schedulerClient.Close(); err != nil {
		logg.Error().Err(err).Msg("scheduler client close failed")
	}
	logg.Info().Msg("scheduler client closed")

	// 2. Redis
	if err := rdb.Close(); err != nil {
		logg.Error().Err(err).Msg("redis close failed")
	}
	logg.Info().Msg("redis disconnected")

	// 3. Database (defer handles this, but explicitly close here for clarity)
	db.Close()

	logg.Info().Msg("server stopped cleanly")
}
