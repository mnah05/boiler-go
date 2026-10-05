package handler

import (
	"context"
	"sync"
	"time"

	"boiler-go/internal/cache"
	"boiler-go/pkg/logger"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

type HealthHandler struct {
	db      *sqlx.DB
	cache   cache.Store
	timeout time.Duration
}

func NewHealthHandler(db *sqlx.DB, cache cache.Store, timeout time.Duration) *HealthHandler {
	return &HealthHandler{db: db, cache: cache, timeout: timeout}
}

// HealthStatus reports downstream dependency state.
type HealthStatus struct {
	Database   string `json:"database" example:"up"`
	RedisCache string `json:"redis_cache" example:"up"`
}

// HealthCheckResponse is the GET /health payload.
type HealthCheckResponse struct {
	Status   HealthStatus `json:"status"`
	Checked  time.Time    `json:"checked"`
	Duration int64        `json:"duration" example:"3"`
}

func checkDependencies(ctx context.Context, db *sqlx.DB, cache cache.Store) (dbStatus, cacheStatus string) {
	var wg sync.WaitGroup
	wg.Add(1)
	var dbErr error
	go func() { defer wg.Done(); dbErr = db.PingContext(ctx) }()
	cacheStatus = "down"
	if cache.Available() {
		cacheStatus = "up"
	}
	wg.Wait()
	dbStatus = "up"
	if dbErr != nil {
		dbStatus = "down"
	}
	return
}

// Check godoc
// @Summary		Health check
// @Description	Returns database and Redis cache status. Returns 503 when the database is down.
// @Tags			health
// @Produce		json
// @Success		200	{object}	handler.HealthCheckResponse
// @Failure		503	{object}	handler.HealthCheckResponse
// @Router			/health [get]
func (h *HealthHandler) Check(c echo.Context) error {
	start := time.Now()
	ctx, cancel := context.WithTimeout(c.Request().Context(), h.timeout)
	defer cancel()
	dbStatus, cacheStatus := checkDependencies(ctx, h.db, h.cache)
	status := 200
	if dbStatus == "down" {
		status = 503
	}
	log := logger.FromContext(c.Request().Context())
	log.Info().Dur("duration", time.Since(start)).Str("database", dbStatus).Str("redis_cache", cacheStatus).Msg("health check completed")
	return c.JSON(status, HealthCheckResponse{Status: HealthStatus{Database: dbStatus, RedisCache: cacheStatus}, Checked: time.Now().UTC(), Duration: time.Since(start).Milliseconds()})
}
