package handler

import (
	"net/http"

	"boiler-go/internal/config"
	custommiddleware "boiler-go/internal/middleware"
	"boiler-go/internal/scheduler"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func NewRouter(log zerolog.Logger, cfg *config.Config, db *pgxpool.Pool, redis *redis.Client, scheduler *scheduler.Client) http.Handler {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposeHeaders:    []string{"Link", "X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	// Use native Echo middleware for request logging and request ID handling
	e.Use(custommiddleware.RequestLogger(log))

	health := NewHealthHandler(db, redis, cfg.HealthCheckTimeout)
	worker := NewWorkerHandler(scheduler)

	e.GET("/health", health.Check)

	// Worker routes
	workerGroup := e.Group("/worker")
	workerGroup.GET("/status", worker.Status)
	workerGroup.POST("/ping", worker.Ping)

	return e
}
