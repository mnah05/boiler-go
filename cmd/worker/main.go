package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boiler-go/internal/config"
	"boiler-go/internal/queue"
	"boiler-go/internal/repository/pool"
	"boiler-go/internal/tasks"
	"boiler-go/pkg/logger"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func main() {
	bootLog := logger.NewProduction("info")

	cfg, err := config.Load()
	if err != nil {
		bootLog.Error().Err(err).Msg("failed to load config")
		os.Exit(1)
	}

	logg, logCleanup, err := logger.NewLogger(cfg, "logs/worker.log")
	if err != nil {
		bootLog.Error().Err(err).Msg("failed to create logger")
		os.Exit(1)
	}
	if logCleanup != nil {
		defer logCleanup()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := pool.Open(ctx, cfg); err != nil {
		logg.Error().Err(err).Msg("failed to initialize database")
		os.Exit(1)
	}
	logg.Info().Msg("database connected")
	defer func() {
		pool.Close()
		logg.Info().Msg("database disconnected")
	}()

	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	redisCtx, redisCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer redisCancel()
	if err := rdb.Ping(redisCtx).Err(); err != nil {
		logg.Error().Err(err).Msg("redis connection failed")
		os.Exit(1)
	}
	logg.Info().Msg("redis connected")
	defer func() {
		if err := rdb.Close(); err != nil {
			logg.Error().Err(err).Msg("redis close failed")
		} else {
			logg.Info().Msg("redis disconnected")
		}
	}()

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: cfg.WorkerConcurrency,
			Queues:      queue.Priorities(),

			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				if n > 6 {
					n = 6
				}
				return time.Duration(1<<uint(n)) * time.Second
			},

			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				taskID := "unknown"
				if rw := task.ResultWriter(); rw != nil {
					taskID = rw.TaskID()
				}
				logg.Error().
					Err(err).
					Str("task_type", task.Type()).
					Str("task_id", taskID).
					Msg("task processing failed")
			}),
		},
	)

	mux := asynq.NewServeMux()

	mux.Use(loggingMiddleware(logg))

	mux.HandleFunc(tasks.TypeWorkerPing, func(ctx context.Context, t *asynq.Task) error {
		var payload tasks.PingTaskPayload
		logEvent := logg.Info()

		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			logEvent.Str("payload_raw", string(t.Payload()))
		} else {
			logEvent.Str("payload", payload.Message)
			if payload.RequestID != "" {
				logEvent.Str("request_id", payload.RequestID)
			}
		}

		logEvent.
			Str("task_type", t.Type()).
			Msg("worker ping task processed - worker is alive!")
		return nil
	})

	workerErrors := make(chan error, 1)

	go func() {
		logg.Info().Msg("worker starting")
		if err := srv.Run(mux); err != nil {
			workerErrors <- fmt.Errorf("worker failed: %w", err)
		} else {
			workerErrors <- nil
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-workerErrors:
		if err != nil {
			logg.Error().Err(err).Msg("worker startup failed")
			os.Exit(1)
		}
		logg.Info().Msg("worker exited cleanly")
	case sig := <-sigChan:
		logg.Info().Str("signal", sig.String()).Msg("shutdown signal received")
	}

	logg.Info().Msg("shutting down worker...")

	srv.Stop()
	logg.Info().Msg("worker stopped accepting new tasks")

	// Drain workerErrors so the srv.Run goroutine never blocks on send.
	go func() {
		<-workerErrors
	}()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.WorkerShutdownTimeout)
	defer shutdownCancel()

	if err := rdb.Ping(shutdownCtx).Err(); err != nil {
		logg.Warn().Err(err).Msg("redis unreachable before shutdown, tasks may not be reclaimed")
	}

	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logg.Error().Interface("panic", r).Msg("worker shutdown panicked")
			}
		}()
		srv.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		logg.Info().Msg("worker shutdown completed gracefully")
	case <-shutdownCtx.Done():
		logg.Warn().Msg("worker shutdown timed out, forcing exit")
		os.Exit(1)
	}

	logg.Info().Msg("worker stopped cleanly")
}

func loggingMiddleware(logg zerolog.Logger) asynq.MiddlewareFunc {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
			start := time.Now()

			taskID := "unknown"
			if rw := task.ResultWriter(); rw != nil {
				taskID = rw.TaskID()
			}

			logg.Info().
				Str("task_type", task.Type()).
				Str("task_id", taskID).
				Msg("task started")

			err := next.ProcessTask(ctx, task)
			duration := time.Since(start)

			if err != nil {
				logg.Error().
					Err(err).
					Str("task_type", task.Type()).
					Str("task_id", taskID).
					Dur("duration", duration).
					Msg("task failed")
			} else {
				logg.Info().
					Str("task_type", task.Type()).
					Str("task_id", taskID).
					Dur("duration", duration).
					Msg("task completed")
			}

			return err
		})
	}
}
