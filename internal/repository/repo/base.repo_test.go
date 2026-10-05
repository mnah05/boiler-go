package repo

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"boiler-go/pkg/logger"

	"github.com/rs/zerolog"
)

// TestLogQueryUsesRequestScopedLogger is why a failed query is now traceable:
// it lands on the logger the request put in the context, so it carries that
// request's ID rather than the repository's construction-time logger.
func TestLogQueryUsesRequestScopedLogger(t *testing.T) {
	var baseSink, requestSink bytes.Buffer
	base := zerolog.New(&baseSink).Level(zerolog.DebugLevel)
	requestScoped := zerolog.New(&requestSink).Level(zerolog.DebugLevel).
		With().Str("request_id", "abc123").Logger()

	r := NewBaseRepo(nil, base)
	ctx := logger.WithContext(context.Background(), requestScoped)

	r.logQuery(ctx, "GetByID", "users", errors.New("connection refused"), time.Now())

	logged := requestSink.String()
	if !strings.Contains(logged, `"request_id":"abc123"`) {
		t.Errorf("query log must carry the request_id, got: %s", logged)
	}
	if !strings.Contains(logged, "connection refused") {
		t.Errorf("query log must carry the failure cause, got: %s", logged)
	}
	if baseSink.Len() != 0 {
		t.Errorf("query log leaked to the base logger: %s", baseSink.String())
	}
}

// TestLogQueryFallsBackToBaseLogger covers background work with no request
// context.
func TestLogQueryFallsBackToBaseLogger(t *testing.T) {
	var baseSink bytes.Buffer
	base := zerolog.New(&baseSink).Level(zerolog.DebugLevel)

	NewBaseRepo(nil, base).logQuery(context.Background(), "List", "users", nil, time.Now())

	if logged := baseSink.String(); !strings.Contains(logged, `"query":"List"`) {
		t.Errorf("want the base logger to receive the query log, got: %s", logged)
	}
}
