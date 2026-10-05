// Package main is the entrypoint for the boiler-go API.
//
// @title			Boiler-Go API
// @version			1.0
// @description		Dockerized Go API starter using Echo, PostgreSQL, sqlc and Redis.
// @host			localhost:8080
// @BasePath		/
// @schemes			http https
//
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				Type "Bearer" followed by a space and your Clerk session token.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boiler-go/internal/cache"
	"boiler-go/internal/config"
	"boiler-go/internal/handler"
	"boiler-go/internal/mailer"
	"boiler-go/internal/repository/pool"
	"boiler-go/pkg/logger"
)

func main() {
	bootLog := logger.NewProduction("info")

	cfg, err := config.Load()
	if err != nil {
		bootLog.Error().Err(err).Msg("failed to load config")
		os.Exit(1)
	}

	logg, logCleanup, err := logger.NewLogger(cfg, "logs/api.log")
	if err != nil {
		bootLog.Error().Err(err).Msg("failed to create logger")
		os.Exit(1)
	}
	if logCleanup != nil {
		defer func() {
			if err := logCleanup(); err != nil {
				logg.Error().Err(err).Msg("log cleanup failed")
			}
		}()
	}
	// Logs emitted without a request context fall back to this logger, so they
	// reach the configured sink rather than a separate default stdout logger.
	logger.SetGlobal(logg)

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()
	if err := pool.Open(dbCtx, cfg); err != nil {
		logg.Error().Err(err).Msg("failed to initialize database")
		os.Exit(1)
	}
	logg.Info().Msg("database connected")
	defer func() {
		pool.Close()
		logg.Info().Msg("database disconnected")
	}()

	dbPool := pool.Get()

	cacheStore := cache.New(cfg, logg)
	logg.Info().Msg("redis cache monitor started")
	defer func() {
		if err := cacheStore.Close(); err != nil {
			logg.Error().Err(err).Msg("redis cache close failed")
		}
	}()

	// The interface is wired now; domain services can depend on it without
	// knowing whether Resend is configured in a given environment.
	_ = mailer.New(cfg)

	router := handler.NewRouter(logg, cfg, dbPool, cacheStore)

	server := &http.Server{
		Addr:           cfg.AppHost + ":" + cfg.AppPort,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logg.Error().Err(err).Msg("server startup failed")
		os.Exit(1)
	case sig := <-sigChan:
		logg.Info().Str("signal", sig.String()).Msg("shutdown signal received")
	}

	logg.Info().Msg("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.APIShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logg.Error().Err(err).Msg("server shutdown failed")
	} else {
		logg.Info().Msg("server shutdown completed gracefully")
	}

	logg.Info().Msg("server stopped cleanly")
}
