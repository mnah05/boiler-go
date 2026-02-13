package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boiler-go/internal/config"
	"boiler-go/internal/handler"
	"boiler-go/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {

	cfg := config.Load()

	logg := logger.New()

	ctx := context.Background()

	// Parse config from connection string
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		logg.Fatal().Err(err).Msg("failed to parse database config")
	}

	// Modify the config
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	dbpool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		logg.Fatal().Err(err).Msg("failed to create database pool")
	}

	if err := dbpool.Ping(ctx); err != nil {
		logg.Fatal().Err(err).Msg("database connection failed")
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

	router := handler.NewRouter(logg, cfg, dbpool, rdb)

	server := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logg.Info().
			Str("port", cfg.AppPort).
			Msg("server starting")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logg.Fatal().Err(err).Msg("server failed to start")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	logg.Info().Msg("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// graceful HTTP shutdown
	if err := server.Shutdown(shutdownCtx); err != nil {
		logg.Error().Err(err).Msg("server shutdown failed")
	}

	// close resources
	dbpool.Close()
	if err := rdb.Close(); err != nil {
		logg.Error().Err(err).Msg("redis close failed")
	}

	logg.Info().Msg("server stopped cleanly")
}
