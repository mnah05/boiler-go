package logger

import (
	"os"
	"sync"

	"github.com/rs/zerolog"
)

var (
	// global is the default logger instance used as fallback.
	// It's initialized lazily on first access.
	global     zerolog.Logger
	globalOnce sync.Once
)

// New creates a new logger with the default configuration.
func New() zerolog.Logger {
	return zerolog.New(os.Stdout).With().Timestamp().Logger().Level(zerolog.InfoLevel)
}

// Global returns the global fallback logger.
// This is used when a request-scoped logger is not available.
func Global() zerolog.Logger {
	globalOnce.Do(func() {
		global = New()
	})
	return global
}
