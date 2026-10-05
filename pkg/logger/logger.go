package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"boiler-go/internal/config"

	"github.com/rs/zerolog"
)

var (
	global    zerolog.Logger
	globalSet bool
	globalMu  sync.RWMutex
)

func NewLogger(cfg *config.Config, defaultFile string) (zerolog.Logger, func() error, error) {
	switch cfg.LogOutput {
	case "file", "both":
		filePath := cfg.LogFile
		if filePath == "" {
			filePath = defaultFile
		}
		logg, cleanup, err := NewWithFile(filePath, cfg.LogOutput == "both", cfg.LogLevel)
		if err != nil {
			return zerolog.Logger{}, nil, fmt.Errorf("failed to create logger: %w", err)
		}
		return logg, cleanup, nil
	default:
		return NewProduction(cfg.LogLevel), func() error { return nil }, nil
	}
}

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

func New() zerolog.Logger {
	return zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
	}).With().Timestamp().Logger()
}

func NewProduction(level string) zerolog.Logger {
	return zerolog.New(os.Stdout).
		With().Timestamp().Logger().
		Level(ParseLevel(level))
}

func NewWithFile(filePath string, console bool, level string) (zerolog.Logger, func() error, error) {
	var writers []io.Writer
	var file *os.File

	if console {
		writers = append(writers, zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05",
		})
	}

	if filePath != "" {
		dir := filepath.Dir(filePath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0750); err != nil {
				return zerolog.Logger{}, nil, fmt.Errorf("create log dir: %w", err)
			}
		}

		var err error
		file, err = os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return zerolog.Logger{}, nil, fmt.Errorf("open log file: %w", err)
		}
		writers = append(writers, file)
	}

	var output io.Writer
	if len(writers) == 1 {
		output = writers[0]
	} else {
		output = zerolog.MultiLevelWriter(writers...)
	}

	logger := zerolog.New(output).With().Timestamp().Logger().Level(ParseLevel(level))

	cleanup := func() error {
		if file != nil {
			return file.Close()
		}
		return nil
	}

	return logger, cleanup, nil
}

// Global returns the process-wide fallback logger.
func Global() zerolog.Logger {
	globalMu.RLock()
	if globalSet {
		defer globalMu.RUnlock()
		return global
	}
	globalMu.RUnlock()

	globalMu.Lock()
	defer globalMu.Unlock()
	if !globalSet {
		global = New().Level(zerolog.InfoLevel)
		globalSet = true
	}
	return global
}

// SetGlobal installs log as the process-wide fallback. Call it once at startup
// so logs emitted without a request context (background jobs, repositories)
// land in the same sink as request logs instead of a separate stdout logger.
func SetGlobal(log zerolog.Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()
	global = log
	globalSet = true
}
