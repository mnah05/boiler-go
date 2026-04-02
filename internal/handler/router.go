package handler

import (
	"net/http"
	"time"

	"boiler-go/internal/config"
	custommiddleware "boiler-go/internal/middleware"
	"boiler-go/internal/scheduler"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func NewRouter(log zerolog.Logger, cfg *config.Config, db *pgxpool.Pool, redis *redis.Client, scheduler *scheduler.Client) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: cfg.CORSAllowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders: []string{"Link", "X-Request-ID"},
		MaxAge:         300,
	}))
	r.Use(custommiddleware.RequestLogger(log))
	r.Use(custommiddleware.MaxBodySize(1 << 20)) // 1MB global limit

	// Health endpoints (excluded from rate limiting)
	health := NewHealthHandler(db, redis, cfg.HealthCheckTimeout)
	worker := NewWorkerHandler(scheduler, db, redis)
	r.Get("/health", health.Check)
	r.Get("/worker/health", worker.Health)

	// Rate-limited API routes
	r.Group(func(r chi.Router) {
		r.Use(httprate.Limit(10, time.Second,
			httprate.WithKeyByIP(),
			httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
				NewErrorResponse(w, http.StatusTooManyRequests, "rate_limit_exceeded", "too many requests")
			}),
		))

		r.Route("/worker", func(r chi.Router) {
			r.Get("/status", worker.Status)
			r.Post("/ping", worker.Ping)
		})

		repoTest := NewRepoTestHandler(db, log)
		r.Route("/repo-test", func(r chi.Router) {
			r.Post("/users", repoTest.CreateUser)
			r.Get("/users", repoTest.ListUsers)
			r.Get("/users/get", repoTest.GetUser)
		})
	})

	return r
}
