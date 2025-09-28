package httpserver

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// TraceMiddleware starts a span for each HTTP request.
func TraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr := otel.Tracer("http.server")
		ctx, span := tr.Start(r.Context(), r.Method+" "+r.URL.Path)
		defer span.End()
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.target", r.URL.Path),
		)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
