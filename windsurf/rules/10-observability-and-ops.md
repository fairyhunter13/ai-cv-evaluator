---
trigger: always_on
---

Ensure first-class observability with logging, metrics, and tracing.

# Note
- See also: `10a-logging.md` and `10b-metrics-and-tracing.md` for deeper, focused guidance.

# Logging
- Use `log/slog` JSON handler.
- Include `trace_id`, `span_id`, and `request_id` in logs.
- No sensitive data; appropriate log levels.

# Metrics (Prometheus)
- HTTP request latency histograms and counters by route/status.
- Queue metrics: queued, processing, completed, failed.
- AI call metrics by provider and outcome; latency.

# Tracing (OpenTelemetry)
- HTTP middleware with tracing; instrument DB, Redis, Qdrant, and outbound HTTP.
- Export via OTLP to collector; view in Jaeger.

# Logging Details
- Log format: JSON with fields: `ts`, `level`, `msg`, `logger`, `service`, `env`, `request_id`, `trace_id`, `span_id`, `route`, `method`, `status`, `latency_ms`, `remote_ip`.
- Error logging:
  - Include `error`, `error_type`, and stable `error_code` aligned with API taxonomy.
  - Log stack traces at `debug` level only; never include in client responses.
- Redaction:
  - Scrub secrets and large payloads; truncate to safe lengths, include `truncated=true` when applied.
- Sampling:
  - Enable log sampling in high traffic (e.g., 1-in-10 for `info`), never sample `error`.

# Metrics Catalog (Prometheus)
- HTTP server:
  - `http_requests_total{route,method,status}` (counter)
  - `http_request_duration_seconds{route,method}` (histogram) with buckets: 0.05, 0.1, 0.25, 0.5, 1, 2, 5
  - `http_response_size_bytes` (summary) optional
- Queue/Jobs:
  - `jobs_enqueued_total{type}` (counter), `jobs_processing{type}` (gauge), `jobs_completed_total{type}`, `jobs_failed_total{type}`
- AI/LLM:
  - `ai_requests_total{provider,operation}` (counter), `ai_request_duration_seconds{provider,operation}` (histogram)
  - `ai_tokens_total{provider,type}` (counter) for prompt/completion tokens if available
- Database/Redis/Qdrant:
  - Client latency histograms and error counters by operation
- Resource:
  - Process CPU/memory and Go runtime metrics (`promhttp` + `expvar` as needed)

# Tracing Propagation
- Use W3C Trace Context (`traceparent`, `tracestate`).
- Inject/extract into HTTP and message queue contexts.
- Record key attributes: route, job_id, cv_id, project_id (non-PII), retry count, upstream status.

# OpenTelemetry Collector Guidance
- Collector receives OTLP from the app and exports to Jaeger/Tempo.
- Minimal example pipeline:
  - Receivers: `otlp`
  - Processors: `batch`, optional `memory_limiter`, `attributes` for scrubbing
  - Exporters: `jaeger`, `prometheusremotewrite` (optional), `logging` for debug
- Configure via env: `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`.

# Dashboards & Alerts
- Dashboards:
  - HTTP performance and errors by route.
  - Job queue depth and failure rate.
  - AI latency and error rate by provider.
- Alerts:
  - High 5xx rate over 5m.
  - Elevated job failures or DLQ growth.
  - AI upstream error spikes (429/5xx) or timeouts.

# Ops Runbooks
- Incident playbooks for:
  - Database outage → fail fast reads/writes, degrade gracefully, and alert.
  - Redis/queue outage → stop accepting new jobs, surface 503, autoscale or repair.
  - AI provider degradation → switch to mock/fallback, reduce concurrency, communicate impact.
- Include steps to collect diagnostics: recent logs, metrics snapshots, traces, and environment.

# SLOs & Alerts (Guidance)
- Example SLIs: p95 latency < 300ms; 5xx rate < 1%.
- Alerts via Prometheus Alertmanager (optional).

# Definition of Done (Observability)
- Traces visible in Jaeger end-to-end.
- Metrics scrapeable; basic dashboards available.
- Logs structured and correlated with traces.
