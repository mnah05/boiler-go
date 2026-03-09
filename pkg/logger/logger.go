package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

var (
	global     zerolog.Logger
	globalOnce sync.Once
)

// ParseLevel converts string to zerolog level.
func ParseLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// New creates a simple stdout logger (console format for dev).
func New() zerolog.Logger {
	return zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
	}).With().Timestamp().Logger()
}

// NewProduction creates a production logger (json to stdout).
// This is what you use in production.
func NewProduction(level string) zerolog.Logger {
	return zerolog.New(os.Stdout).
		With().Timestamp().Logger().
		Level(ParseLevel(level))
}

// NewWithFile creates a logger with file output.
// filePath: where to write logs (e.g., "logs/app.log")
// console: also print to stdout?
// Note: The file is opened but never closed - acceptable for long-running processes,
// but for proper cleanup, this should be refactored to return a closer function.
func NewWithFile(filePath string, console bool, level string) (zerolog.Logger, error) {
	var writers []io.Writer

	// Console output (pretty)
	if console {
		writers = append(writers, zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05",
		})
	}

	// File output (json)
	if filePath != "" {
		dir := filepath.Dir(filePath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return zerolog.Logger{}, fmt.Errorf("create log dir: %w", err)
			}
		}

		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return zerolog.Logger{}, fmt.Errorf("open log file: %w", err)
		}
		// Note: File is intentionally not closed - zerolog will write to it.
		// For production, use stdout logging (12-factor) instead of files.
		writers = append(writers, file)
	}

	var output io.Writer
	if len(writers) == 1 {
		output = writers[0]
	} else {
		output = zerolog.MultiLevelWriter(writers...)
	}

	return zerolog.New(output).With().Timestamp().Logger().Level(ParseLevel(level)), nil
}

// Global returns a fallback logger.
func Global() zerolog.Logger {
	globalOnce.Do(func() {
		global = New()
	})
	return global
}
