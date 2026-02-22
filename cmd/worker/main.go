package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boiler-go/internal/config"
	"boiler-go/internal/db"
	"boiler-go/pkg/logger"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
)

// task types
const (
	TypeHealthCheck = "system:health_check"
	TypeWorkerPing  = "worker:ping"
)

func main() {
	cfg := config.Load()
	logg := logger.New()

	// Initialize database pool
	if err := db.Open(cfg); err != nil {
		logg.Fatal().Err(err).Msg("failed to initialize database")
	}
	logg.Info().Msg("database connected")

	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 10, // worker concurrency
			// queue priorities (higher weight = higher priority)
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			// StrictPriority: true, // uncomment to always process higher priority queues first

			// Exponential backoff retry strategy
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				// 1s, 2s, 4s, 8s, 16s...
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

	// Add logging middleware
	mux.Use(loggingMiddleware(logg))

	// health check handler (kept for backward compatibility)
	mux.HandleFunc(TypeHealthCheck, func(ctx context.Context, t *asynq.Task) error {
		logg.Info().
			Str("task_type", t.Type()).
			Msg("health check task processed")
		return nil
	})

	// worker ping handler - used by API to verify worker is alive
	mux.HandleFunc(TypeWorkerPing, func(ctx context.Context, t *asynq.Task) error {
		payload := string(t.Payload())
		if payload == "" {
			payload = "no payload"
		}
		logg.Info().
			Str("task_type", t.Type()).
			Str("payload", payload).
			Msg("worker ping task processed - worker is alive!")
		return nil
	})

	workerErrors := make(chan error, 1)

	go func() {
		logg.Info().Msg("worker starting")
		if err := srv.Run(mux); err != nil {
			workerErrors <- fmt.Errorf("worker failed to start: %w", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-workerErrors:
		logg.Fatal().Err(err).Msg("worker startup failed")
	case sig := <-stop:
		logg.Info().Str("signal", sig.String()).Msg("shutdown signal received")
	}

	logg.Info().Msg("shutting down worker...")

	// Graceful shutdown: First stop accepting new tasks, then wait for in-flight tasks
	// Stop() stops accepting new tasks immediately
	srv.Stop()
	logg.Info().Msg("worker stopped accepting new tasks")

	// Shutdown with timeout enforcement for in-flight tasks
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.WorkerShutdownTimeout)
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

	// Close database connection
	db.Close()

	logg.Info().Msg("worker stopped cleanly")
}

// loggingMiddleware logs task execution duration and success/failure
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
