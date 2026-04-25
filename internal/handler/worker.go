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
	if r.Body != http.NoBody {
		var body PingRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			log.Error().Err(err).Msg("failed to decode ping request")
			if wErr := NewErrorResponse(w, http.StatusBadRequest, "bad_request", "invalid request body"); wErr != nil {
				log.Error().Err(wErr).Msg("failed to write error response")
			}
			return
		}

		if err := validator.ValidateStruct(&body); err != nil {
			validationErrors := validator.GetValidationErrors(err)
			if wErr := NewErrorResponseWithDetails(w, http.StatusBadRequest, "validation_error", "validation failed", validationErrors); wErr != nil {
				log.Error().Err(wErr).Msg("failed to write error response")
			}
			return
		}

		payloadMsg = body.Message
	}

	if payloadMsg == "" {
		payloadMsg = "ping from API"
	}

	now := time.Now().UTC()
	payload := tasks.PingTaskPayload{
		Message:   payloadMsg,
		RequestID: requestID,
		QueuedAt:  now,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal ping payload")
		if wErr := NewErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to create task payload"); wErr != nil {
			log.Error().Err(wErr).Msg("failed to write error response")
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	taskID, err := h.scheduler.EnqueueWithID(ctx, tasks.TypeWorkerPing, payloadBytes,
		asynq.Queue(queue.QueueDefault),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to enqueue worker ping task")
		if wErr := NewErrorResponse(w, http.StatusServiceUnavailable, "service_unavailable", "failed to enqueue task"); wErr != nil {
			log.Error().Err(wErr).Msg("failed to write error response")
		}
		return
	}

	log.Info().
		Str("task_id", taskID).
		Str("task_type", tasks.TypeWorkerPing).
		Str("request_id", requestID).
		Msg("worker ping task enqueued")

	if wErr := NewSuccessResponse(w, http.StatusAccepted, PingResponse{
		TaskID:   taskID,
		TaskType: tasks.TypeWorkerPing,
		QueuedAt: now,
	}, "task queued successfully"); wErr != nil {
		log.Error().Err(wErr).Msg("failed to write success response")
	}
}

func (h *WorkerHandler) Status(w http.ResponseWriter, r *http.Request) {
	log := logger.FromChiContext(r.Context())
	requestID := middleware.GetReqID(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	overall := http.StatusOK
	dbStatus, redisStatus := checkDependencies(ctx, h.db, h.redis)

	status := map[string]string{
		"redis":    dbStatus,
		"database": redisStatus,
	}
	if dbStatus == "down" || redisStatus == "down" {
		overall = http.StatusServiceUnavailable
	}

	if err := WriteJSON(w, overall, map[string]any{
		"request_id": requestID,
		"redis":      status["redis"],
		"database":   status["database"],
		"queues":     queue.Names(),
		"note":       "Use POST /worker/ping to test task processing",
	}); err != nil {
		log.Error().Err(err).Msg("failed to write status response")
	}
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

	overall := http.StatusOK
	dbStatus, redisStatus := checkDependencies(ctx, h.db, h.redis)

	status := map[string]string{
		"database": dbStatus,
		"redis":    redisStatus,
	}
	if dbStatus == "down" || redisStatus == "down" {
		overall = http.StatusServiceUnavailable
	}

	duration := time.Since(start)

	log.Info().
		Dur("duration", duration).
		Str("database", status["database"]).
		Str("redis", status["redis"]).
		Msg("worker health check completed")

	if err := WriteJSON(w, overall, HealthResponse{
		Status:   status,
		Checked:  time.Now().UTC(),
		Duration: duration.Milliseconds(),
	}); err != nil {
		log.Error().Err(err).Msg("failed to write health check response")
	}
}
