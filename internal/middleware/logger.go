package middleware

import (
	"time"

	"boiler-go/pkg/logger"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

func RequestLogger(base zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			reqID := c.Response().Header().Get(echo.HeaderXRequestID)
			path := c.Path()
			if path == "" {
				path = c.Request().URL.Path
			}

			reqLogger := base.With().
				Str("request_id", reqID).
				Str("method", c.Request().Method).
				Str("path", path).
				Logger()

			c.SetRequest(c.Request().WithContext(logger.WithContext(c.Request().Context(), reqLogger)))
			err := next(c)

			reqLogger.Info().
				Dur("duration", time.Since(start)).
				Int("status", c.Response().Status).
				Msg("request completed")
			return err
		}
	}
}
