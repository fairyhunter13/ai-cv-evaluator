---
trigger: always_on
---

Comprehensive guidance for metrics and tracing.

# Metrics (Prometheus)
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

# Tracing (OpenTelemetry)
- HTTP middleware with tracing; instrument DB, Redis, Qdrant, and outbound HTTP.
- Export via OTLP to collector; view in Jaeger/Tempo.

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

# SLOs & Alerts (Guidance)
- Example SLIs: p95 latency < 300ms; 5xx rate < 1%.
- Alerts via Prometheus Alertmanager (optional).

# Minimal Observability Stack (Compose)
- Services (example):
  ```yaml
  otel-collector:
    image: otel/opentelemetry-collector:0.98.0
    command: ["--config=/etc/otel-collector-config.yaml"]
    volumes:
      - ./deploy/otel-collector-config.yaml:/etc/otel-collector-config.yaml:ro
    ports: ["4317:4317", "4318:4318"] # gRPC & HTTP

  jaeger:
    image: jaegertracing/all-in-one:1.57
    ports: ["16686:16686"] # UI

  prometheus:
    image: prom/prometheus:v2.53.0
    volumes:
      - ./deploy/prometheus.yml:/etc/prometheus/prometheus.yml:ro
    ports: ["9090:9090"]

  loki:
    image: grafana/loki:2.9.4
    command: ["-config.file=/etc/loki/config.yaml"]
    volumes:
      - ./deploy/loki-config.yaml:/etc/loki/config.yaml:ro
    ports: ["3100:3100"]

  promtail:
    image: grafana/promtail:2.9.4
    command: ["-config.file=/etc/promtail/config.yaml"]
    volumes:
      - /var/log:/var/log:ro
      - ./deploy/promtail-config.yaml:/etc/promtail/config.yaml:ro

  grafana:
    image: grafana/grafana:11.1.0
    ports: ["3000:3000"]
    depends_on: [prometheus, loki]
  ```
- App env:
  - `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317`
  - `OTEL_SERVICE_NAME=cv-job-matching`
  - Ensure logs are JSON with `trace_id`, `span_id`, `request_id` (see `10a-logging.md`).

# End-to-End Tracking (How-To)
- Propagate W3C trace context across HTTP and queue boundaries.
- Generate or accept `X-Request-Id` and inject it into responses and logs.
- In Grafana:
  - Add Tempo (or Jaeger via data source) and Loki data sources.
  - Configure a dashboard with:
    - Panel 1: Trace view (by `trace_id`).
    - Panel 2: Logs for trace — Loki query filtering on `trace_id` label.
- Ensure promtail parses JSON logs and extracts labels `trace_id`, `request_id` for correlation.

# Definition of Done (Metrics & Tracing)
- Metrics scrapeable; dashboards available.
- Traces visible end-to-end across components.
