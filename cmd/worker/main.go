package main

import (
	"context"
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

// simple test task type
const TypeHealthCheck = "system:health_check"

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

	// very basic test handler
	mux.HandleFunc(TypeHealthCheck, func(ctx context.Context, t *asynq.Task) error {
		logg.Info().
			Str("task_type", t.Type()).
			Msg("worker is working")
		return nil
	})

	go func() {
		logg.Info().Msg("worker starting")
		if err := srv.Run(mux); err != nil {
			logg.Fatal().Err(err).Msg("worker failed to start")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logg.Info().Msg("shutting down worker...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown will wait for in-flight tasks to complete or until timeout
	srv.Shutdown()

	// Close database connection
	db.Close()

	// Wait for context to ensure clean shutdown
	<-shutdownCtx.Done()

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
