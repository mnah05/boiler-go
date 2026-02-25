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

// OutputConfig defines where logs should be written.
type OutputConfig struct {
	// FilePath is the path to the log file. If empty, no file logging.
	FilePath string
	// Stdout enables console output.
	Stdout bool
	// StdoutOnly disables file output and only logs to stdout.
	StdoutOnly bool
}

// New creates a new logger with the default configuration (stdout only).
func New() zerolog.Logger {
	return NewWithOutput(OutputConfig{Stdout: true, StdoutOnly: true})
}

// NewWithOutput creates a new logger with configurable output destinations.
func NewWithOutput(cfg OutputConfig) zerolog.Logger {
	var writers []io.Writer

	// Add stdout if enabled or if stdout-only mode
	if cfg.Stdout || cfg.StdoutOnly {
		writers = append(writers, zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05",
		})
	}

	// Add file output if configured and not stdout-only
	if !cfg.StdoutOnly && cfg.FilePath != "" {
		// Ensure log directory exists
		dir := filepath.Dir(cfg.FilePath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				// Fall back to stdout with error if can't create directory
				fmt.Fprintf(os.Stderr, "failed to create log directory: %v\n", err)
				return zerolog.New(os.Stdout).With().Timestamp().Logger().Level(zerolog.InfoLevel)
			}
		}

		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			// Fall back to stdout with error if can't open file
			fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
			return zerolog.New(os.Stdout).With().Timestamp().Logger().Level(zerolog.InfoLevel)
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

	return zerolog.New(output).With().Timestamp().Logger().Level(zerolog.InfoLevel)
}

// Global returns the global fallback logger.
// This is used when a request-scoped logger is not available.
func Global() zerolog.Logger {
	globalOnce.Do(func() {
		global = New()
	})
	return global
}
