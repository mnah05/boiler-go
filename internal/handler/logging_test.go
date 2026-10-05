package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	custommiddleware "boiler-go/internal/middleware"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
)

// newLoggingTestRouter mirrors the middleware order in NewRouter so these tests
// exercise the real error and panic logging path.
func newLoggingTestRouter(sink io.Writer) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = HTTPErrorHandler
	e.Use(
		echomw.RequestID(),
		echomw.RecoverWithConfig(echomw.RecoverConfig{LogErrorFunc: custommiddleware.PanicLogger()}),
		custommiddleware.RequestLogger(zerolog.New(sink).Level(zerolog.DebugLevel)),
	)
	return e
}

func serve(e *echo.Echo, method, target, body string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// TestServerErrorLogsRealStatusReasonAndRequestID is the core guarantee: a
// failed request is logged with its real status, its cause, and an ID the
// caller can quote.
func TestServerErrorLogsRealStatusReasonAndRequestID(t *testing.T) {
	var sink bytes.Buffer
	e := newLoggingTestRouter(&sink)
	e.GET("/fail", func(c echo.Context) error {
		return NewEchoError(http.StatusInternalServerError, "internal_error", "database is down")
	})

	rec := serve(e, http.MethodGet, "/fail", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("client status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	logs := sink.String()
	if !strings.Contains(logs, `"status":500`) {
		t.Errorf("completion line must report the real status, got:\n%s", logs)
	}
	if !strings.Contains(logs, `"level":"error"`) {
		t.Errorf("5xx must be logged at error level, got:\n%s", logs)
	}
	if !strings.Contains(logs, "database is down") {
		t.Errorf("failure reason missing from logs:\n%s", logs)
	}

	var payload ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if payload.RequestID == "" {
		t.Fatal("error body must carry request_id")
	}
	if got := rec.Header().Get(echo.HeaderXRequestID); got != payload.RequestID {
		t.Errorf("body request_id %q != response header %q", payload.RequestID, got)
	}
	if !strings.Contains(logs, payload.RequestID) {
		t.Errorf("request_id %q from the body not found in logs:\n%s", payload.RequestID, logs)
	}
}

// TestClientErrorLogsRealStatus guards the original bug: a returned error was
// logged as status 200 because Echo renders it after the middleware unwinds.
func TestClientErrorLogsRealStatus(t *testing.T) {
	var sink bytes.Buffer
	e := newLoggingTestRouter(&sink)
	e.POST("/users", func(c echo.Context) error {
		return NewEchoError(http.StatusBadRequest, "validation_error", "email is required")
	})

	rec := serve(e, http.MethodPost, "/users", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("client status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	logs := sink.String()
	if !strings.Contains(logs, `"status":400`) {
		t.Errorf("want status 400 in logs, got:\n%s", logs)
	}
	if !strings.Contains(logs, `"level":"warn"`) {
		t.Errorf("4xx should be logged at warn level, got:\n%s", logs)
	}
	if strings.Contains(logs, `"status":200`) {
		t.Errorf("a 400 must never be logged as 200, got:\n%s", logs)
	}
}

// TestRouterErrorLogsRealStatus covers errors raised by Echo itself rather than
// by a handler.
func TestRouterErrorLogsRealStatus(t *testing.T) {
	var sink bytes.Buffer
	e := newLoggingTestRouter(&sink)

	rec := serve(e, http.MethodGet, "/does-not-exist", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("client status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if logs := sink.String(); !strings.Contains(logs, `"status":404`) {
		t.Errorf("unmatched route must log status 404, got:\n%s", logs)
	}
}

// TestWrappedErrorKeepsStatus ensures status resolution unwraps the error, as
// repositories and services wrap failures with context.
func TestWrappedErrorKeepsStatus(t *testing.T) {
	var sink bytes.Buffer
	e := newLoggingTestRouter(&sink)
	e.GET("/conflict", func(c echo.Context) error {
		return fmt.Errorf("service: create user: %w", NewEchoError(http.StatusConflict, "conflict", "email already exists"))
	})

	rec := serve(e, http.MethodGet, "/conflict", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("client status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if logs := sink.String(); !strings.Contains(logs, `"status":409`) {
		t.Errorf("wrapped error must log status 409, got:\n%s", logs)
	}
}

// TestPanicIsLoggedWithStackAndRequestID covers the case that previously
// produced no structured log at all.
func TestPanicIsLoggedWithStackAndRequestID(t *testing.T) {
	var sink bytes.Buffer
	e := newLoggingTestRouter(&sink)
	e.GET("/panic", func(c echo.Context) error { panic("nil map write") })

	rec := serve(e, http.MethodGet, "/panic", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("client status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	logs := sink.String()
	for _, want := range []string{`"level":"error"`, `"panic":"nil map write"`, `"stack":`, "goroutine", `"status":500`} {
		if !strings.Contains(logs, want) {
			t.Errorf("panic log missing %q, got:\n%s", want, logs)
		}
	}

	if id := rec.Header().Get(echo.HeaderXRequestID); id == "" || !strings.Contains(logs, id) {
		t.Errorf("panic log must carry request_id %q, got:\n%s", id, logs)
	}
	if body := rec.Body.String(); strings.Contains(body, "nil map write") {
		t.Errorf("panic value leaked to the client: %s", body)
	}
}

// TestSuccessLoggedOnceAtInfo keeps signal clean: one line per successful
// request, at info level.
func TestSuccessLoggedOnceAtInfo(t *testing.T) {
	var sink bytes.Buffer
	e := newLoggingTestRouter(&sink)
	e.GET("/ok", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	rec := serve(e, http.MethodGet, "/ok", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("client status = %d, want %d", rec.Code, http.StatusOK)
	}

	logs := sink.String()
	if got := strings.Count(logs, "request completed"); got != 1 {
		t.Errorf("want exactly 1 completion line, got %d:\n%s", got, logs)
	}
	if !strings.Contains(logs, `"status":200`) || !strings.Contains(logs, `"level":"info"`) {
		t.Errorf("success should log status 200 at info level, got:\n%s", logs)
	}
}
