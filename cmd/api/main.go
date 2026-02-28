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
	"github.com/rs/zerolog"
)

// newLogger creates a logger based on the configuration.
// defaultFile is used when LOG_FILE is not set and LOG_OUTPUT is "file" or "both".
func newLogger(cfg *config.Config, defaultFile string) zerolog.Logger {
	outputCfg := logger.OutputConfig{}

	switch cfg.LogOutput {
	case "stdout":
		outputCfg.Stdout = true
		outputCfg.StdoutOnly = true
	case "file":
		outputCfg.Stdout = false
		outputCfg.StdoutOnly = false
		outputCfg.FilePath = cfg.LogFile
		if outputCfg.FilePath == "" {
			outputCfg.FilePath = defaultFile
		}
	case "both":
		outputCfg.Stdout = true
		outputCfg.StdoutOnly = false
		outputCfg.FilePath = cfg.LogFile
		if outputCfg.FilePath == "" {
			outputCfg.FilePath = defaultFile
		}
	}

	return logger.NewWithOutput(outputCfg)
}

func main() {
	// Load config first with basic logger
	cfg := config.Load(logger.New())

	// Create logger based on configuration
	logg := newLogger(cfg, "logs/api.log")
	ctx := context.Background()

	// Initialize database pool with timeout context
	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()
	if err := db.Open(dbCtx, cfg); err != nil {
		logg.Fatal().Err(err).Msg("failed to initialize database")
	}
	logg.Info().Msg("database connected")
	defer db.Close()

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
	defer schedulerClient.Close()

	router := handler.NewRouter(logg, cfg, db.Get(), rdb, schedulerClient)

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

	// close resources in reverse order of initialization
	if err := rdb.Close(); err != nil {
		logg.Error().Err(err).Msg("redis close failed")
	}

	logg.Info().Msg("server stopped cleanly")
}
