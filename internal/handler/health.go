package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"boiler-go/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

func NewHealthHandler(db *pgxpool.Pool, redis *redis.Client) *HealthHandler {
	return &HealthHandler{
		db:    db,
		redis: redis,
	}
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
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

	response := map[string]any{
		"status":  status,
		"checked": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(overall)

	json.NewEncoder(w).Encode(response)
}
