---
trigger: always_on
---

First-class logging guidance for the service.

# Logging (Overview)
- Use `log/slog` JSON handler.
- Include `trace_id`, `span_id`, and `request_id` in logs.
- Avoid sensitive data; use appropriate log levels.

# Logging Details
- Format fields (recommended):
  - `ts`, `level`, `msg`, `logger`, `service`, `env`,
  - `request_id`, `trace_id`, `span_id`, `route`, `method`, `status`, `latency_ms`, `remote_ip`.
- Error logging:
  - Include `error`, `error_type`, and stable `error_code` aligned with API taxonomy.
  - Log stack traces at `debug` only; never include them in client responses.
- Redaction:
  - Scrub secrets and large payloads; truncate to safe lengths; include `truncated=true` when applied.
  - Never log raw LLM prompts or completions in production; log only token counts and status/latency metrics. See `04-ai-llm-and-rag-pipeline.md` → Chain-of-Thought (CoT) Handling.
- Sampling:
  - Enable sampling in high traffic (e.g., 1-in-10 for `info`), never sample `error`.

# Correlation (Logs ↔ Traces ↔ Requests)
- Ensure every HTTP request has a `request_id` (header `X-Request-Id` or generated) and an active OpenTelemetry span.
- Inject `trace_id` and `span_id` into every log entry by binding the logger to the context that carries the span.
- Include `request_id` in logs and responses for user support and cross-system correlation.

# Example (slog + OTEL context)
```go
// In middleware, bind a request-scoped logger with trace & request IDs
func LoggerMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        reqID := r.Header.Get("X-Request-Id")
        if reqID == "" { reqID = newReqID() }

        // Extract OTEL span context
        spanCtx := trace.SpanContextFromContext(r.Context())
        traceID := spanCtx.TraceID().String()
        spanID := spanCtx.SpanID().String()

        logger := slog.Default().With(
            slog.String("request_id", reqID),
            slog.String("trace_id", traceID),
            slog.String("span_id", spanID),
        )
        ctx := context.WithValue(r.Context(), loggerKey{}, logger)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

# Definition of Done (Logging)
- Structured JSON logs are emitted across all routes and workers.
- Error logs include structured codes and relevant context.
- No secrets or PII leakage verified by code review/tests.
