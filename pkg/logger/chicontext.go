package logger

import (
	"context"

	"github.com/rs/zerolog"
)

type chiContextKey string

const chiLoggerKey chiContextKey = "logger"

func FromChiContext(ctx context.Context) zerolog.Logger {
	if log, ok := ctx.Value(chiLoggerKey).(zerolog.Logger); ok {
		return log
	}
	return Global()
}

func WithChiContext(ctx context.Context, log zerolog.Logger) context.Context {
	return context.WithValue(ctx, chiLoggerKey, log)
}
