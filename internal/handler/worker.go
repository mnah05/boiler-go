package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"boiler-go/internal/scheduler"
	"boiler-go/pkg/logger"

	"github.com/hibiken/asynq"
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
func (h *WorkerHandler) Ping(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	// Parse optional message from request body
	var req PingRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Error().Err(err).Msg("failed to decode ping request")
			http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
			return
		}
	}

	// Default message if not provided
	payload := []byte(req.Message)
	if len(payload) == 0 {
		payload = []byte("ping from API")
	}

	// Enqueue the ping task
	taskID, err := h.scheduler.EnqueueWithID(r.Context(), "worker:ping", payload,
		asynq.Queue("default"),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to enqueue worker ping task")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "failed to enqueue task",
			"details": err.Error(),
		})
		return
	}

	log.Info().
		Str("task_id", taskID).
		Str("task_type", "worker:ping").
		Msg("worker ping task enqueued")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(PingResponse{
		Success:  true,
		TaskID:   taskID,
		TaskType: "worker:ping",
		QueuedAt: time.Now().UTC(),
		Message:  "Task queued successfully. Check worker logs to verify processing.",
	})
}

// Status returns the current worker/queue status
// GET /worker/status
func (h *WorkerHandler) Status(w http.ResponseWriter, r *http.Request) {
	// Since we don't have direct queue inspection without Redis commands,
	// we return basic info about the scheduler
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"scheduler": "connected",
		"queues":    []string{"critical", "default", "low"},
		"note":      "Use POST /worker/ping to test task processing",
	})
}
