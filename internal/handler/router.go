package handler

import (
	"net/http"

	"boiler-go/internal/config"
	custommiddleware "boiler-go/internal/middleware"
	"boiler-go/internal/scheduler"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func NewRouter(log zerolog.Logger, cfg *config.Config, db *pgxpool.Pool, redis *redis.Client, scheduler *scheduler.Client) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(custommiddleware.RequestLogger(log))

	health := NewHealthHandler(db, redis, cfg.HealthCheckTimeout)
	worker := NewWorkerHandler(scheduler)

	r.Get("/health", health.Check)

	// Worker routes
	r.Route("/worker", func(r chi.Router) {
		r.Get("/status", worker.Status)
		r.Post("/ping", worker.Ping)
	})

	return r
}
