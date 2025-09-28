package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
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
			}
			spanCtx := trace.SpanContextFromContext(r.Context())
			logger := slog.Default().With(
				slog.String("request_id", reqID),
				slog.String("trace_id", spanCtx.TraceID().String()),
				slog.String("span_id", spanCtx.SpanID().String()),
			)
			ctx := context.WithValue(r.Context(), loggerKey{}, logger)
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

func newReqID() string {
	return time.Now().UTC().Format("20060102150405.000000000")
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
			lg.Info("http_access",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Duration("duration_ms", dur),
				slog.String("request_id", r.Header.Get("X-Request-Id")),
				slog.String("trace_id", spanCtx.TraceID().String()),
				slog.String("span_id", spanCtx.SpanID().String()),
			)
		})
	}
}
