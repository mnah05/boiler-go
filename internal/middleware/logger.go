package middleware

import (
	"net/http"
	"time"

	"boiler-go/pkg/logger"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

func RequestLogger(base zerolog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Get request ID from Chi's built-in middleware
			reqID := middleware.GetReqID(r.Context())

			// Get route pattern from Chi context
			rctx := chi.RouteContext(r.Context())
			path := r.URL.Path
			if rctx != nil {
				path = rctx.RoutePattern()
			}

			// Create request-scoped logger with request ID
			reqLogger := base.With().
				Str("request_id", reqID).
				Str("method", r.Method).
				Str("path", path).
				Logger()

			// Inject logger into Chi context
			ctx := logger.WithChiContext(r.Context(), reqLogger)

			// Use response writer to capture status code
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Log request completion
			reqLogger.Info().
				Dur("duration", time.Since(start)).
				Int("status", rw.statusCode).
				Msg("request completed")
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
