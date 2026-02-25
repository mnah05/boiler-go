package logger

import (
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

// FromEchoContext extracts the logger from an echo context.
// It first checks for a logger stored with the "logger" key,
// then falls back to extracting from the standard context.
func FromEchoContext(c echo.Context) zerolog.Logger {
	// First try to get from echo's context store
	if log, ok := c.Get("logger").(zerolog.Logger); ok {
		return log
	}
	// Fall back to standard context extraction
	return FromContext(c.Request().Context())
}
