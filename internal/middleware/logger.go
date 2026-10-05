package middleware

import (
	"errors"
	"net/http"
	"time"

	"boiler-go/pkg/logger"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

// statusCoder is implemented by application errors that carry an HTTP status,
// such as handler.APIError. It lets this package report the true status of a
// failed request without importing the handler package.
type statusCoder interface {
	StatusCode() int
}

// RequestLogger builds a request-scoped logger, stores it on the request
// context so handlers and repositories can add fields to the same log stream,
// and emits exactly one completion line per request.
func RequestLogger(base zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			reqLogger := base.With().
				Str("request_id", requestID(c)).
				Str("method", c.Request().Method).
				Str("path", requestPath(c)).
				Logger()

			c.SetRequest(c.Request().WithContext(logger.WithContext(c.Request().Context(), reqLogger)))

			err := next(c)

			// Echo renders a returned error *after* this middleware unwinds, so
			// c.Response().Status is still the default 200 at this point. Read
			// the status off the error instead of trusting the response, or
			// every failure gets logged as a success.
			status := c.Response().Status
			if err != nil {
				status = statusFromError(err)
			}

			event := reqLogger.Info()
			switch {
			case status >= http.StatusInternalServerError:
				event = reqLogger.Error()
			case status >= http.StatusBadRequest:
				event = reqLogger.Warn()
			}

			event.
				Int("status", status).
				Dur("duration", time.Since(start)).
				Str("remote_ip", c.RealIP()).
				Msg("request completed")

			return err
		}
	}
}

// PanicLogger returns a RecoverConfig.LogErrorFunc that records a recovered
// panic as structured JSON on the request-scoped logger, stack included.
//
// It must return a non-nil error: Recover only hands the error to the
// centralized HTTPErrorHandler when LogErrorFunc does. Returning the panic
// value itself would leak internals to the client, so it returns a plain 500.
func PanicLogger() func(c echo.Context, err error, stack []byte) error {
	return func(c echo.Context, err error, stack []byte) error {
		// The request-scoped logger already carries request_id, method and
		// path; repeating them here would emit duplicate JSON keys.
		log := logger.FromContext(c.Request().Context())
		log.Error().
			Int("status", http.StatusInternalServerError).
			Str("panic", err.Error()).
			Str("stack", string(stack)).
			Msg("panic recovered")

		return echo.ErrInternalServerError
	}
}

// statusFromError recovers the HTTP status an error will be rendered with,
// unwrapping as needed.
func statusFromError(err error) int {
	var coder statusCoder
	if errors.As(err, &coder) {
		if status := coder.StatusCode(); status > 0 {
			return status
		}
	}

	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) && httpErr.Code > 0 {
		return httpErr.Code
	}

	return http.StatusInternalServerError
}

// requestID prefers the ID Echo echoed on the response and falls back to the
// inbound header for responses that have not been committed yet.
func requestID(c echo.Context) string {
	if id := c.Response().Header().Get(echo.HeaderXRequestID); id != "" {
		return id
	}
	return c.Request().Header.Get(echo.HeaderXRequestID)
}

// requestPath is the route pattern when the router matched one, which keeps
// cardinality low, and the raw URL otherwise (unmatched routes).
func requestPath(c echo.Context) string {
	if path := c.Path(); path != "" {
		return path
	}
	return c.Request().URL.Path
}
