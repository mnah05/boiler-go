package logger

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// TestSetGlobalRoutesFallbackLogs verifies that logs emitted without a request
// context reach the configured sink instead of a separate default logger.
func TestSetGlobalRoutesFallbackLogs(t *testing.T) {
	var sink bytes.Buffer
	SetGlobal(zerolog.New(&sink).Level(zerolog.DebugLevel))

	fallback := FromContext(context.Background())
	fallback.Info().Msg("no request context")

	if logged := sink.String(); !strings.Contains(logged, "no request context") {
		t.Errorf("fallback logs must reach the configured sink, got: %s", logged)
	}
}

func TestFromContextOKReportsPresence(t *testing.T) {
	if _, ok := FromContextOK(context.Background()); ok {
		t.Error("a context without a logger must report none")
	}

	scoped := zerolog.New(&bytes.Buffer{}).Level(zerolog.DebugLevel)
	ctx := WithContext(context.Background(), scoped)
	if _, ok := FromContextOK(ctx); !ok {
		t.Error("a context carrying a logger must report one")
	}
}
