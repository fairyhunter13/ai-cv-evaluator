// Package httpserver contains HTTP handlers and middleware.
//
// It provides REST API endpoints for the application including
// file upload, evaluation triggering, and result retrieval.
// The package follows clean architecture principles and provides
// a clear separation between HTTP concerns and business logic.
package httpserver

import (
	"context"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/oklog/ulid/v2"
	"go.opentelemetry.io/otel/trace"

	obsctx "github.com/fairyhunter13/ai-cv-evaluator/internal/observability"
)

// Recoverer ensures panics don't crash the server and responds 500 safely.
func Recoverer() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					slog.Error("panic recovered", slog.Any("recover", rec))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestID injects a request id and correlates with tracing ids.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get("X-Request-Id")
			if reqID == "" {
				reqID = newReqID()
				r.Header.Set("X-Request-Id", reqID)
			}
			spanCtx := trace.SpanContextFromContext(r.Context())
			logger := slog.Default().With(
				slog.String("request_id", reqID),
				slog.String("trace_id", spanCtx.TraceID().String()),
				slog.String("span_id", spanCtx.SpanID().String()),
			)
			ctx := context.WithValue(r.Context(), loggerKey{}, logger)
			ctx = obsctx.ContextWithLogger(ctx, logger)
			ctx = obsctx.ContextWithRequestID(ctx, reqID)
			w.Header().Set("X-Request-Id", reqID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TimeoutMiddleware adds a deadline to the request context.
func TimeoutMiddleware(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, http.StatusText(http.StatusGatewayTimeout))
	}
}

// SecurityHeaders adds strict security headers suitable for a JSON API.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		// HSTS should be set at the reverse proxy/edge in HTTPS environments
		next.ServeHTTP(w, r)
	})
}

type loggerKey struct{}

// LoggerFrom extracts the request-scoped logger from the context or returns the default logger.
func LoggerFrom(r *http.Request) *slog.Logger {
	if v := r.Context().Value(loggerKey{}); v != nil {
		if lg, ok := v.(*slog.Logger); ok {
			return lg
		}
	}
	return slog.Default()
}

var ulidEntropy = ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0) //nolint:gosec // Weak random is sufficient for ULID entropy.

func newReqID() string {
	// Generate a ULID-based request ID for better global uniqueness and
	// lexicographic ordering while remaining URL/header friendly.
	id, err := ulid.New(ulid.Timestamp(time.Now()), ulidEntropy)
	if err != nil {
		// Fallback to timestamp-based ID if ULID generation fails for any reason.
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return id.String()
}

// AccessLog logs basic request/response information at info level.
func AccessLog() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			dur := time.Since(start)
			spanCtx := trace.SpanContextFromContext(r.Context())
			lg := LoggerFrom(r)
			// Derive the same route pattern used by Prometheus metrics so that
			// Loki route labels can line up with the Prometheus route label.
			var route string
			if rc := chi.RouteContext(r.Context()); rc != nil {
				route = rc.RoutePattern()
			}
			if route == "" {
				route = r.URL.Path
			}
			statusCode := ww.Status()
			statusText := http.StatusText(statusCode)
			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("route", route),
				slog.Int("status", statusCode),
				slog.String("status_text", statusText),
				slog.Duration("duration_ms", dur),
				slog.String("request_id", r.Header.Get("X-Request-Id")),
				slog.String("trace_id", spanCtx.TraceID().String()),
				slog.String("span_id", spanCtx.SpanID().String()),
			}
			// Log at appropriate level based on status code
			switch {
			case statusCode >= 500:
				lg.LogAttrs(r.Context(), slog.LevelError, "http_access", attrs...)
			case statusCode >= 400:
				lg.LogAttrs(r.Context(), slog.LevelWarn, "http_access", attrs...)
			default:
				lg.LogAttrs(r.Context(), slog.LevelInfo, "http_access", attrs...)
			}
		})
	}
}
