package logger

import (
	"context"

	"github.com/rs/zerolog"
)

type contextKey string

const loggerKey contextKey = "logger"

// FromContext returns the request-scoped logger, falling back to the global
// logger when the context carries none.
func FromContext(ctx context.Context) zerolog.Logger {
	if log, ok := FromContextOK(ctx); ok {
		return log
	}
	g := Global()
	g.Debug().Msg("no logger in context, falling back to global logger")
	return g
}

// FromContextOK reports whether ctx carries a logger, without falling back.
// Callers that already hold a sensible base logger use this to prefer the
// request-scoped one only when it exists.
func FromContextOK(ctx context.Context) (zerolog.Logger, bool) {
	log, ok := ctx.Value(loggerKey).(zerolog.Logger)
	return log, ok
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
