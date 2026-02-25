package middleware

import (
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

// RequestLogger returns an Echo middleware that logs requests and injects
// a request-scoped logger with request_id into the context.
func RequestLogger(base zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			// Get or generate request ID
			reqID := c.Request().Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = uuid.NewString()
			}

			// Set request ID on response header
			c.Response().Header().Set("X-Request-ID", reqID)

			// Also set it on the request header so it's available to wrapped handlers
			c.Request().Header.Set("X-Request-ID", reqID)

			// Create request-scoped logger
			reqLogger := base.With().
				Str("request_id", reqID).
				Str("method", c.Request().Method).
				Str("path", c.Request().URL.Path).
				Logger()

			// Inject logger into echo.Context
			c.Set("logger", reqLogger)

			// Execute next handler
			err := next(c)

			// Log request completion
			reqLogger.Info().
				Dur("duration", time.Since(start)).
				Int("status", c.Response().Status).
				Msg("request completed")

			return err
		}
	}
}
