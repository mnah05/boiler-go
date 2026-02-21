package handler

import (
	"net/http"

	"boiler-go/internal/config"
	custommiddleware "boiler-go/internal/middleware"
	"boiler-go/internal/scheduler"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func NewRouter(log zerolog.Logger, cfg *config.Config, db *pgxpool.Pool, redis *redis.Client, scheduler *scheduler.Client) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)
	r.Use(custommiddleware.RequestLogger(log))

	health := NewHealthHandler(db, redis, scheduler, cfg.HealthCheckTimeout)

	r.Get("/health", health.Check)

	return r
}
