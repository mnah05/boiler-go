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
	"boiler-go/internal/handler"
	"boiler-go/internal/repository/pool"
	"boiler-go/internal/scheduler"
	"boiler-go/pkg/logger"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

func main() {
	bootLog := logger.New()

	cfg, err := config.Load()
	if err != nil {
		bootLog.Fatal().Err(err).Msg("failed to load config")
	}

	logg, logCleanup := logger.NewLogger(cfg, "logs/api.log")
	if logCleanup != nil {
		defer logCleanup()
	}
	ctx := context.Background()

	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()
	if err := pool.Open(dbCtx, cfg); err != nil {
		logg.Fatal().Err(err).Msg("failed to initialize database")
	}
	logg.Info().Msg("database connected")

	dbPool := pool.Get()
	if dbPool == nil {
		logg.Fatal().Msg("database pool is nil")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		PoolSize:     cfg.RedisPoolSize,
		MinIdleConns: cfg.RedisMinIdleConns,
		DialTimeout:  cfg.RedisDialTimeout,
		ReadTimeout:  cfg.RedisReadTimeout,
		WriteTimeout: cfg.RedisWriteTimeout,
	})

	redisCtx, redisCancel := context.WithTimeout(ctx, 5*time.Second)
	defer redisCancel()
	if err := rdb.Ping(redisCtx).Err(); err != nil {
		logg.Fatal().Err(err).Msg("redis connection failed")
	}
	logg.Info().Msg("redis connected")

	schedulerClient := scheduler.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	logg.Info().Msg("scheduler client initialized")

	router := handler.NewRouter(logg, cfg, dbPool, rdb, schedulerClient)

	server := &http.Server{
		Addr:           ":" + cfg.AppPort,
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
		logg.Fatal().Err(err).Msg("server startup failed")
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

	if err := schedulerClient.Close(); err != nil {
		logg.Error().Err(err).Msg("scheduler client close failed, retrying")
		time.Sleep(100 * time.Millisecond)
		if retryErr := schedulerClient.Close(); retryErr != nil {
			logg.Error().Err(retryErr).Msg("scheduler client close failed on retry")
		} else {
			logg.Info().Msg("scheduler client closed on retry")
		}
	} else {
		logg.Info().Msg("scheduler client closed")
	}

	if err := rdb.Close(); err != nil {
		logg.Error().Err(err).Msg("redis close failed, retrying")
		time.Sleep(100 * time.Millisecond)
		if retryErr := rdb.Close(); retryErr != nil {
			logg.Error().Err(retryErr).Msg("redis close failed on retry")
		} else {
			logg.Info().Msg("redis disconnected on retry")
		}
	} else {
		logg.Info().Msg("redis disconnected")
	}

	pool.Close()
	logg.Info().Msg("database disconnected")

	logg.Info().Msg("server stopped cleanly")
}
