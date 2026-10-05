package handler

import (
	"context"
	"errors"
	"net/http"

	"boiler-go/pkg/logger"

	"github.com/labstack/echo/v4"
)

type ErrorResponse struct {
	Error     string `json:"error" example:"bad_request"`
	Message   string `json:"message,omitempty" example:"invalid request body"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"request_id,omitempty" example:"3f2b1c9d8e7a6b5c4d3e2f1a0b9c8d7e"`
}
type SuccessResponse struct {
	Success bool   `json:"success" example:"true"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty" example:"user created"`
}
type APIError struct {
	Status int
	Body   ErrorResponse
}

func (e *APIError) Error() string { return e.Body.Message }

// StatusCode lets middleware recover the HTTP status of an error without
// importing this package.
func (e *APIError) StatusCode() int { return e.Status }

func NewEchoError(status int, code, message string) error {
	return &APIError{Status: status, Body: ErrorResponse{Error: code, Message: message}}
}
func NewEchoErrorWithDetails(status int, code, message string, details any) error {
	return &APIError{Status: status, Body: ErrorResponse{Error: code, Message: message, Details: details}}
}

// HTTPErrorHandler renders every error as the shared JSON envelope, echoing the
// request ID so a caller can quote it, and records the error on the
// request-scoped logger so failures are traceable to a cause. It is the single
// place a failed request is recorded, which is what makes 5xx visible even for
// handlers that do not log their own errors.
func HTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	requestID := c.Response().Header().Get(echo.HeaderXRequestID)
	status, body := errorResponse(err, requestID)

	logError(c, err, status)

	_ = c.JSON(status, body)
}

// errorResponse maps an error to the status and the client-safe body used to
// render it. Internal details stay in the logs and never reach the caller.
func errorResponse(err error, requestID string) (int, ErrorResponse) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		body := apiErr.Body
		body.RequestID = requestID
		return apiErr.Status, body
	}

	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.Code >= http.StatusInternalServerError {
			return httpErr.Code, ErrorResponse{Error: "internal_error", Message: "internal server error", RequestID: requestID}
		}

		// 401/403 are raised by authenticating middleware, which supplies a
		// client-safe message. Every other Echo error keeps generic text so
		// internals never leak.
		switch httpErr.Code {
		case http.StatusUnauthorized:
			return httpErr.Code, ErrorResponse{Error: "unauthorized", Message: echoMessage(httpErr, "unauthorized"), RequestID: requestID}
		case http.StatusForbidden:
			return httpErr.Code, ErrorResponse{Error: "forbidden", Message: echoMessage(httpErr, "forbidden"), RequestID: requestID}
		}

		return httpErr.Code, ErrorResponse{Error: "http_error", Message: "request failed", RequestID: requestID}
	}

	return http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "internal server error", RequestID: requestID}
}

// echoMessage returns an Echo error's message when it is a non-empty string,
// falling back otherwise.
func echoMessage(err *echo.HTTPError, fallback string) string {
	if message, ok := err.Message.(string); ok && message != "" {
		return message
	}
	return fallback
}

func logError(c echo.Context, err error, status int) {
	log := logger.FromContext(c.Request().Context())

	// A caller that hung up is not a server fault, but it surfaces here as a
	// failed request. Label it so it is not mistaken for a real 5xx.
	if errors.Is(c.Request().Context().Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		log.Warn().Int("status", status).Err(err).Msg("request aborted by client")
		return
	}

	switch {
	case status >= http.StatusInternalServerError:
		log.Error().Int("status", status).Err(err).Msg("request failed")
	case status >= http.StatusBadRequest:
		log.Warn().Int("status", status).Err(err).Msg("request failed")
	default:
		log.Info().Int("status", status).Err(err).Msg("request failed")
	}
}
