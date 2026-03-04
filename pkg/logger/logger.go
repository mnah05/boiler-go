package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog"
)

var (
	// global is the default logger instance used as fallback.
	// It's initialized lazily on first access.
	global     zerolog.Logger
	globalOnce sync.Once
)

// New creates a new logger with the default configuration (stdout only).
func New() zerolog.Logger {
	logger, _ := NewWithOutput("", true)
	return logger
}

// NewWithOutput creates a new logger with configurable output destinations.
// Returns an error if file logging is enabled but the file/directory cannot be created.
func NewWithOutput(filePath string, enableConsole bool) (zerolog.Logger, error) {
	var writers []io.Writer

	// Add stdout if enabled
	if enableConsole {
		writers = append(writers, zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05",
		})
	}

	// Add file output if configured
	if filePath != "" {
		// Ensure log directory exists
		dir := filepath.Dir(filePath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return zerolog.Logger{}, fmt.Errorf("failed to create log directory: %w", err)
			}
		}

		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return zerolog.Logger{}, fmt.Errorf("failed to open log file: %w", err)
		}

		// Note: We don't close the file here - the caller should manage the lifecycle
		// or we can use a sync.Once to close on exit if needed
		writers = append(writers, file)
	}

	var output io.Writer
	if len(writers) == 0 {
		// Default to stdout if nothing configured
		output = os.Stdout
	} else if len(writers) == 1 {
		output = writers[0]
	} else {
		// Multi-writer for both stdout and file
		output = zerolog.MultiLevelWriter(writers...)
	}

	return zerolog.New(output).With().Timestamp().Logger().Level(zerolog.InfoLevel), nil
}

// Global returns the global fallback logger.
// This is used when a request-scoped logger is not available.
func Global() zerolog.Logger {
	globalOnce.Do(func() {
		global = New()
	})
	return global
}
