package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"boiler-go/internal/queue"
	"boiler-go/internal/scheduler"
	"boiler-go/internal/tasks"
	"boiler-go/internal/validator"
	"boiler-go/pkg/logger"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type WorkerHandler struct {
	scheduler *scheduler.Client
	db        *pgxpool.Pool
	redis     *redis.Client
}

func NewWorkerHandler(scheduler *scheduler.Client, db *pgxpool.Pool, redis *redis.Client) *WorkerHandler {
	return &WorkerHandler{
		scheduler: scheduler,
		db:        db,
		redis:     redis,
	}
}

type PingRequest struct {
	Message string `json:"message,omitempty" validate:"omitempty,max=500"`
}

type PingResponse struct {
	TaskID   string    `json:"task_id"`
	TaskType string    `json:"task_type"`
	QueuedAt time.Time `json:"queued_at"`
}

func (h *WorkerHandler) Ping(w http.ResponseWriter, r *http.Request) {
	log := logger.FromChiContext(r.Context())
	requestID := middleware.GetReqID(r.Context())

	var payloadMsg string
	if r.ContentLength > 0 {
		var body PingRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			log.Error().Err(err).Msg("failed to decode ping request")
			NewErrorResponse(w, http.StatusBadRequest, "bad_request", "invalid request body")
			return
		}

		if err := validator.ValidateStruct(&body); err != nil {
			errors := validator.GetValidationErrors(err)
			NewErrorResponseWithDetails(w, http.StatusBadRequest, "validation_error", "validation failed", errors)
			return
		}

		payloadMsg = body.Message
	}

	if payloadMsg == "" {
		payloadMsg = "ping from API"
	}

	payload := tasks.PingTaskPayload{
		Message:   payloadMsg,
		RequestID: requestID,
		QueuedAt:  time.Now().UTC(),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal ping payload")
		NewErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to create task payload")
		return
	}

	taskID, err := h.scheduler.EnqueueWithID(r.Context(), tasks.TypeWorkerPing, payloadBytes,
		asynq.Queue(queue.QueueDefault),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to enqueue worker ping task")
		NewErrorResponse(w, http.StatusServiceUnavailable, "service_unavailable", "failed to enqueue task")
		return
	}

	log.Info().
		Str("task_id", taskID).
		Str("task_type", tasks.TypeWorkerPing).
		Str("request_id", requestID).
		Msg("worker ping task enqueued")

	NewSuccessResponse(w, http.StatusAccepted, PingResponse{
		TaskID:   taskID,
		TaskType: tasks.TypeWorkerPing,
		QueuedAt: time.Now().UTC(),
	}, "task queued successfully")
}

func (h *WorkerHandler) Status(w http.ResponseWriter, r *http.Request) {
	log := logger.FromChiContext(r.Context())
	requestID := middleware.GetReqID(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	status := map[string]string{
		"redis":    "unknown",
		"database": "unknown",
	}
	overall := http.StatusOK

	if err := h.redis.Ping(ctx).Err(); err != nil {
		log.Error().Err(err).Msg("redis health check failed")
		status["redis"] = "disconnected"
		overall = http.StatusServiceUnavailable
	} else {
		status["redis"] = "connected"
	}

	if err := h.db.Ping(ctx); err != nil {
		log.Error().Err(err).Msg("database health check failed")
		status["database"] = "disconnected"
		overall = http.StatusServiceUnavailable
	} else {
		status["database"] = "connected"
	}

	WriteJSON(w, overall, map[string]any{
		"request_id": requestID,
		"redis":      status["redis"],
		"database":   status["database"],
		"queues":     queue.Names(),
		"note":       "Use POST /worker/ping to test task processing",
	})
}

type HealthResponse struct {
	Status   map[string]string `json:"status"`
	Checked  time.Time         `json:"checked"`
	Duration int64             `json:"duration"`
}

func (h *WorkerHandler) Health(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	log := logger.FromChiContext(r.Context())

	status := map[string]string{
		"database": "up",
		"redis":    "up",
	}
	overall := http.StatusOK

	if err := h.db.Ping(ctx); err != nil {
		log.Error().Err(err).Msg("database health check failed")
		status["database"] = "down"
		overall = http.StatusServiceUnavailable
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		log.Error().Err(err).Msg("redis health check failed")
		status["redis"] = "down"
		overall = http.StatusServiceUnavailable
	}

	duration := time.Since(start)

	log.Info().
		Dur("duration", duration).
		Str("database", status["database"]).
		Str("redis", status["redis"]).
		Msg("worker health check completed")

	WriteJSON(w, overall, HealthResponse{
		Status:   status,
		Checked:  time.Now().UTC(),
		Duration: duration.Milliseconds(),
	})
}
