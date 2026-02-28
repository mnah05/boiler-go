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
	"boiler-go/internal/db"
	"boiler-go/internal/queue"
	"boiler-go/internal/tasks"
	"boiler-go/pkg/logger"

	"github.com/hibiken/asynq"
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

// PingTaskPayload mirrors the structure from internal/handler/worker.go
type PingTaskPayload struct {
	Message   string    `json:"message"`
	RequestID string    `json:"request_id"`
	QueuedAt  time.Time `json:"queued_at"`
}

func main() {
	// Load config first with basic logger
	cfg := config.Load(logger.New())

	// Create logger based on configuration
	logg := newLogger(cfg, "logs/worker.log")

	// Initialize database pool with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.Open(ctx, cfg); err != nil {
		logg.Fatal().Err(err).Msg("failed to initialize database")
	}
	logg.Info().Msg("database connected")
	defer db.Close()

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
			Queues: queue.Priorities(),
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

	// worker ping handler - used by API to verify worker is alive
	mux.HandleFunc(tasks.TypeWorkerPing, func(ctx context.Context, t *asynq.Task) error {
		// Parse payload for correlation ID
		var payload PingTaskPayload
		logEvent := logg.Info()

		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			// Fallback to raw payload if parsing fails
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
			workerErrors <- fmt.Errorf("worker failed to start: %w", err)
		}
	}()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-workerErrors:
		logg.Fatal().Err(err).Msg("worker startup failed")
	case sig := <-sigChan:
		logg.Info().Str("signal", sig.String()).Msg("shutdown signal received")
	}

	logg.Info().Msg("shutting down worker...")

	// Stop accepting new tasks
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
