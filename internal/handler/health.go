package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"boiler-go/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	db      *pgxpool.Pool
	redis   *redis.Client
	timeout time.Duration
}

func NewHealthHandler(db *pgxpool.Pool, redis *redis.Client, timeout time.Duration) *HealthHandler {
	return &HealthHandler{
		db:      db,
		redis:   redis,
		timeout: timeout,
	}
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	log := logger.FromContext(ctx)

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

	response := map[string]any{
		"status":   status,
		"checked":  time.Now().UTC(),
		"duration": duration.Milliseconds(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(overall)

	// Use a separate writer to avoid partial writes on error
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(response); err != nil {
		log.Error().Err(err).Msg("failed to encode health check response")
		// Can't write error response since headers already sent
		return
	}

	// Write the buffered response
	if _, err := w.Write(buf.Bytes()); err != nil {
		log.Error().Err(err).Msg("failed to write health check response")
	}

	// Log health check duration for monitoring/debugging
	log.Debug().
		Dur("duration", duration).
		Str("database", status["database"]).
		Str("redis", status["redis"]).
		Msg("health check completed")
}
