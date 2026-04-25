package handler

import (
	"context"
	"net/http"
	"sync"
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

// checkDependencies pings DB and Redis concurrently and returns their statuses.
func checkDependencies(ctx context.Context, db *pgxpool.Pool, rdb *redis.Client) (dbStatus, redisStatus string) {
	var wg sync.WaitGroup
	wg.Add(2)

	var dbErr, redisErr error

	go func() {
		defer wg.Done()
		dbErr = db.Ping(ctx)
	}()

	go func() {
		defer wg.Done()
		redisErr = rdb.Ping(ctx).Err()
	}()

	wg.Wait()

	if dbErr != nil {
		dbStatus = "down"
	} else {
		dbStatus = "up"
	}

	if redisErr != nil {
		redisStatus = "down"
	} else {
		redisStatus = "up"
	}

	return
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	log := logger.FromChiContext(r.Context())

	overall := http.StatusOK
	dbStatus, redisStatus := checkDependencies(ctx, h.db, h.redis)

	if dbStatus == "down" || redisStatus == "down" {
		overall = http.StatusServiceUnavailable
	}

	duration := time.Since(start)

	log.Info().
		Dur("duration", duration).
		Str("database", dbStatus).
		Str("redis", redisStatus).
		Msg("health check completed")

	if err := WriteJSON(w, overall, map[string]any{
		"status": map[string]string{
			"database": dbStatus,
			"redis":    redisStatus,
		},
		"checked":  time.Now().UTC(),
		"duration": duration.Milliseconds(),
	}); err != nil {
		log.Error().Err(err).Msg("failed to write health check response")
	}
}
