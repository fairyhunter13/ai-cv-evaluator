package observability

import (
	"context"
	"log/slog"
	"testing"
)

func TestContextWithLoggerAndLoggerFromContext(t *testing.T) {
	lg := slog.Default()

	baseCtx := context.Background()

	// Attaching a logger should return a derived context
	ctxWithLogger := ContextWithLogger(baseCtx, lg)
	if ctxWithLogger == baseCtx {
		t.Fatal("expected a derived context when attaching a logger")
	}

	// Logger should round-trip through context
	if got := LoggerFromContext(ctxWithLogger); got != lg {
		t.Fatalf("LoggerFromContext did not return original logger, got %v", got)
	}

	// When logger is nil, original context should be returned unchanged
	if got := ContextWithLogger(baseCtx, nil); got != baseCtx {
		t.Fatal("expected original context when logger is nil")
	}

	// Default logger should be returned when context has no logger
	if got := LoggerFromContext(context.Background()); got == nil {
		t.Fatal("expected default logger for empty context")
	}
}

func TestContextWithRequestIDAndRequestIDFromContext(t *testing.T) {
	ctx := context.Background()
	reqID := "req-123"
	ctxWithID := ContextWithRequestID(ctx, reqID)

	if ctxWithID == ctx {
		t.Fatal("expected a derived context when setting request ID")
	}

	if got := RequestIDFromContext(ctxWithID); got != reqID {
		t.Fatalf("RequestIDFromContext() = %q, want %q", got, reqID)
	}

	// Missing request ID should return empty string
	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Fatalf("expected empty string when no request ID present, got %q", got)
	}
}
