package observability

import (
	"context"
	"log/slog"
)

// loggerContextKey is the private context key used to store a *slog.Logger.
type loggerContextKey struct{}

// requestIDContextKey is the private context key used to store the originating
// HTTP request_id so that background workers and deeper layers can correlate
// their logs with the original request.
type requestIDContextKey struct{}

// ContextWithLogger attaches a non-nil logger to the context.
func ContextWithLogger(ctx context.Context, lg *slog.Logger) context.Context {
	if ctx == nil || lg == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerContextKey{}, lg)
}

// LoggerFromContext returns the logger stored in the context or the default
// slog logger when none is present.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if v := ctx.Value(loggerContextKey{}); v != nil {
		if lg, ok := v.(*slog.Logger); ok && lg != nil {
			return lg
		}
	}
	return slog.Default()
}

// ContextWithRequestID stores a non-empty request_id in the context so that
// downstream layers (queue workers, AI client, etc.) can correlate their logs
// with the originating HTTP request.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil || requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// RequestIDFromContext retrieves the request_id from the context, or an empty
// string when none is present.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value(requestIDContextKey{}); v != nil {
		if rid, ok := v.(string); ok {
			return rid
		}
	}
	return ""
}
