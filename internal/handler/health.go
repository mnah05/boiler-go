package handler

import (
	"context"
	"net/http"
	"time"

	"boiler-go/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
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

// Check handles GET /health using Echo's context.
func (h *HealthHandler) Check(c echo.Context) error {
	start := time.Now()
	req := c.Request()

	ctx, cancel := context.WithTimeout(req.Context(), h.timeout)
	defer cancel()

	log := logger.FromEchoContext(c)

	status := echo.Map{
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

	response := echo.Map{
		"status":   status,
		"checked":  time.Now().UTC(),
		"duration": duration.Milliseconds(),
	}

	// Log health check completion at Info level for operational visibility
	dbStatus, _ := status["database"].(string)
	redisStatus, _ := status["redis"].(string)
	log.Info().
		Dur("duration", duration).
		Str("database", dbStatus).
		Str("redis", redisStatus).
		Msg("health check completed")

	return c.JSON(overall, response)
}
