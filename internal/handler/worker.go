package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"boiler-go/internal/queue"
	"boiler-go/internal/scheduler"
	"boiler-go/internal/tasks"
	"boiler-go/pkg/logger"

	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

type WorkerHandler struct {
	scheduler *scheduler.Client
}

func NewWorkerHandler(scheduler *scheduler.Client) *WorkerHandler {
	return &WorkerHandler{
		scheduler: scheduler,
	}
}

// PingRequest represents the request body for worker ping
type PingRequest struct {
	Message string `json:"message,omitempty"`
}

// PingTaskPayload is the payload for the worker ping task, including correlation ID.
type PingTaskPayload struct {
	Message   string    `json:"message"`
	RequestID string    `json:"request_id"`
	QueuedAt  time.Time `json:"queued_at"`
}

// PingResponse represents the response from worker ping
type PingResponse struct {
	Success  bool      `json:"success"`
	TaskID   string    `json:"task_id"`
	TaskType string    `json:"task_type"`
	QueuedAt time.Time `json:"queued_at"`
	Message  string    `json:"message,omitempty"`
}

// Ping enqueues a test task to verify worker is processing jobs
// POST /worker/ping
func (h *WorkerHandler) Ping(c echo.Context) error {
	req := c.Request()
	res := c.Response()

	log := logger.FromEchoContext(c)

	// Extract request ID for correlation
	requestID := req.Header.Get("X-Request-ID")

	// Limit request body size to 1MB
	req.Body = http.MaxBytesReader(res, req.Body, 1<<20)

	// Parse optional message from request body
	var payloadMsg string
	if req.ContentLength > 0 {
		var body PingRequest
		if err := c.Bind(&body); err != nil {
			log.Error().Err(err).Msg("failed to decode ping request")
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
		}
		payloadMsg = body.Message
	}

	// Default message if not provided
	if payloadMsg == "" {
		payloadMsg = "ping from API"
	}

	// Build payload with correlation ID
	payload := PingTaskPayload{
		Message:   payloadMsg,
		RequestID: requestID,
		QueuedAt:  time.Now().UTC(),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal ping payload")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create task payload",
		})
	}

	// Enqueue the ping task
	taskID, err := h.scheduler.EnqueueWithID(req.Context(), tasks.TypeWorkerPing, payloadBytes,
		asynq.Queue(queue.QueueDefault),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to enqueue worker ping task")
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error":   "failed to enqueue task",
			"details": err.Error(),
		})
	}

	log.Info().
		Str("task_id", taskID).
		Str("task_type", tasks.TypeWorkerPing).
		Str("request_id", requestID).
		Msg("worker ping task enqueued")

	return c.JSON(http.StatusAccepted, PingResponse{
		Success:  true,
		TaskID:   taskID,
		TaskType: tasks.TypeWorkerPing,
		QueuedAt: time.Now().UTC(),
		Message:  "Task queued successfully. Check worker logs to verify processing.",
	})
}

// Status returns the current worker/queue status
// GET /worker/status
func (h *WorkerHandler) Status(c echo.Context) error {
	// Return queue info from shared package to ensure consistency
	return c.JSON(http.StatusOK, map[string]any{
		"scheduler": "connected",
		"queues":    queue.Names(),
		"note":      "Use POST /worker/ping to test task processing",
	})
}
