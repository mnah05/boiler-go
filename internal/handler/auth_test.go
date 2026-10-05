package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	custommiddleware "boiler-go/internal/middleware"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/labstack/echo/v4"
)

// clerkAuthRouter mounts ClerkAuth exactly as router.go does and anchors a
// protected route that reads the Clerk user ID from the request context.
func clerkAuthRouter(sink *bytes.Buffer) *echo.Echo {
	clerk.SetKey("sk_test_placeholder")
	e := newLoggingTestRouter(sink)
	protected := e.Group("", custommiddleware.ClerkAuth())
	protected.GET("/me", func(c echo.Context) error {
		clerkID, ok := custommiddleware.ClerkUserIDFromContext(c.Request().Context())
		if !ok {
			return NewEchoError(http.StatusUnauthorized, "unauthorized", "missing claims")
		}
		return c.JSON(http.StatusOK, map[string]string{"clerk_id": clerkID})
	})
	return e
}

func requestWithAuth(e *echo.Echo, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	if authorization != "" {
		req.Header.Set(echo.HeaderAuthorization, authorization)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// TestClerkAuthMissingTokenReturnsEnvelopeAndLogs is the regression guard for
// the SDK's bare 403: an unauthenticated request must get a 401 carrying the
// shared envelope, and the reason must reach the log.
func TestClerkAuthMissingTokenReturnsEnvelopeAndLogs(t *testing.T) {
	var sink bytes.Buffer
	e := clerkAuthRouter(&sink)

	rec := requestWithAuth(e, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}

	var payload ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("error body must be JSON: %v (body=%q)", err, rec.Body.String())
	}
	if payload.Error != "unauthorized" {
		t.Errorf("error code = %q, want %q", payload.Error, "unauthorized")
	}
	if payload.Message == "" {
		t.Error("error body must explain the failure")
	}
	if payload.RequestID == "" {
		t.Fatal("error body must carry request_id")
	}
	if got := rec.Header().Get(echo.HeaderXRequestID); got != payload.RequestID {
		t.Errorf("body request_id %q != response header %q", payload.RequestID, got)
	}

	logs := sink.String()
	want := []string{`"status":401`, `"level":"warn"`, "missing or malformed authorization header", payload.RequestID}
	for _, w := range want {
		if !strings.Contains(logs, w) {
			t.Errorf("log missing %q, got:\n%s", w, logs)
		}
	}
}

// TestClerkAuthMalformedTokenReturnsEnvelope covers a token that is present but
// cannot be decoded, which is rejected without any network call.
func TestClerkAuthMalformedTokenReturnsEnvelope(t *testing.T) {
	var sink bytes.Buffer
	e := clerkAuthRouter(&sink)

	rec := requestWithAuth(e, "Bearer not-a-jwt")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}

	var payload ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("error body must be JSON: %v (body=%q)", err, rec.Body.String())
	}
	if payload.Error != "unauthorized" || payload.RequestID == "" {
		t.Errorf("want unauthorized with a request_id, got %+v", payload)
	}
	if logs := sink.String(); !strings.Contains(logs, "invalid or expired token") {
		t.Errorf("log must state the reason, got:\n%s", logs)
	}
}

// TestClerkAuthRejectsNonBearerScheme makes sure another auth scheme is never
// mistaken for a session token.
func TestClerkAuthRejectsNonBearerScheme(t *testing.T) {
	var sink bytes.Buffer
	e := clerkAuthRouter(&sink)

	rec := requestWithAuth(e, "Basic dXNlcjpwYXNz")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if logs := sink.String(); !strings.Contains(logs, `"status":401`) {
		t.Errorf("non-bearer auth must be logged as 401, got:\n%s", logs)
	}
}
