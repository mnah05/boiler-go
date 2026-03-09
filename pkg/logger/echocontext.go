package logger

import (
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

// contextKey is the key used to store/retrieve the logger from echo.Context.
const contextKey = "logger"

// FromEchoContext retrieves the logger from Echo's context.
// If no logger is found in the context, it falls back to the global logger
// instead of a no-op logger to ensure logs are never silently dropped.
func FromEchoContext(c echo.Context) zerolog.Logger {
	// First try to get from echo's context store
	if log, ok := c.Get(contextKey).(zerolog.Logger); ok {
		return log
	}
	// Fall back to global logger if missing (never silently drop logs)
	return Global()
}

// SetInEchoContext stores the logger in Echo's context.
func SetInEchoContext(c echo.Context, log zerolog.Logger) {
	c.Set(contextKey, log)
}
