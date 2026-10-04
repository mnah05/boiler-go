package logger

import (
	"context"

	"github.com/rs/zerolog"
)

type contextKey string

const loggerKey contextKey = "logger"

func FromContext(ctx context.Context) zerolog.Logger {
	if log, ok := ctx.Value(loggerKey).(zerolog.Logger); ok {
		return log
	}
	g := Global()
	g.Debug().Msg("no logger in context, falling back to global logger")
	return g
}

func WithContext(ctx context.Context, log zerolog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, log)
}

// FromChiContext and WithChiContext are retained temporarily for callers using
// a net/http handler outside Echo.
func FromChiContext(ctx context.Context) zerolog.Logger { return FromContext(ctx) }
func WithChiContext(ctx context.Context, log zerolog.Logger) context.Context {
	return WithContext(ctx, log)
}
