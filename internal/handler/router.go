package handler

import (
	"net/http"
	"time"

	_ "boiler-go/docs"
	"boiler-go/internal/cache"
	"boiler-go/internal/config"
	custommiddleware "boiler-go/internal/middleware"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	echoSwagger "github.com/swaggo/echo-swagger"
)

func NewRouter(log zerolog.Logger, cfg *config.Config, db *sqlx.DB, cache cache.Store) *echo.Echo {
	clerk.SetKey(cfg.ClerkSecretKey)

	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = HTTPErrorHandler
	e.Use(middleware.RequestID(), middleware.RecoverWithConfig(middleware.RecoverConfig{LogErrorFunc: custommiddleware.PanicLogger()}), echo.WrapMiddleware(custommiddleware.SecurityHeaders(custommiddleware.SecurityConfig{HSTSEnabled: cfg.SecurityHSTSEnabled})), middleware.CORSWithConfig(middleware.CORSConfig{AllowOrigins: cfg.CORSAllowedOrigins, AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions}, AllowHeaders: []string{echo.HeaderAccept, echo.HeaderAuthorization, echo.HeaderContentType, echo.HeaderXRequestID}, ExposeHeaders: []string{"Link", echo.HeaderXRequestID}, MaxAge: 300}), custommiddleware.RequestLogger(log), echo.WrapMiddleware(custommiddleware.MaxBodySize(1<<20)))
	health := NewHealthHandler(db, cache, cfg.HealthCheckTimeout)
	e.GET("/health", health.Check)
	e.GET("/swagger/*", echoSwagger.WrapHandler)
	webhooks := NewWebhookHandler(db, cfg.ClerkWebhookSecret, log)
	e.POST("/webhooks/clerk", webhooks.HandleClerk)
	api := e.Group("", middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{Rate: 10, Burst: 10, ExpiresIn: time.Second})))
	protected := api.Group("", echo.WrapMiddleware(custommiddleware.ClerkAuth()))
	repoTest := NewRepoTestHandler(db, log)
	protected.GET("/me", repoTest.GetMe)
	protected.GET("/repo-test/users", repoTest.ListUsers)
	protected.GET("/repo-test/users/get", repoTest.GetUser)
	return e
}
