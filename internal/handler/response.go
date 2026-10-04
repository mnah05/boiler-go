package handler

import "github.com/labstack/echo/v4"

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Details any    `json:"details,omitempty"`
}
type SuccessResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}
type APIError struct {
	Status int
	Body   ErrorResponse
}

func (e *APIError) Error() string { return e.Body.Message }
func NewEchoError(status int, code, message string) error {
	return &APIError{Status: status, Body: ErrorResponse{Error: code, Message: message}}
}
func NewEchoErrorWithDetails(status int, code, message string, details any) error {
	return &APIError{Status: status, Body: ErrorResponse{Error: code, Message: message, Details: details}}
}
func HTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	if apiErr, ok := err.(*APIError); ok {
		_ = c.JSON(apiErr.Status, apiErr.Body)
		return
	}
	if httpErr, ok := err.(*echo.HTTPError); ok {
		_ = c.JSON(httpErr.Code, ErrorResponse{Error: "http_error", Message: "request failed"})
		return
	}
	_ = c.JSON(500, ErrorResponse{Error: "internal_error", Message: "internal server error"})
}
