# Observability Runbook

This document describes the observability stack, dashboards, and operational procedures for monitoring and troubleshooting the AI CV Evaluator system.

## Stack Overview

| Component   | Purpose                          | Access URL (Production)                     |
|-------------|----------------------------------|---------------------------------------------|
| **Grafana** | Dashboards, alerts, log exploration | `https://ai-cv-evaluator.web.id/grafana/` |
| **Prometheus** | Metrics collection & storage   | `https://ai-cv-evaluator.web.id/prometheus/` |
| **Loki**    | Centralized log aggregation      | Via Grafana Explore                         |
| **Jaeger**  | Distributed tracing              | `https://ai-cv-evaluator.web.id/jaeger/`    |
| **Promtail**| Log shipper (Docker containers → Loki) | Internal only                         |
| **OTEL Collector** | Trace pipeline (app → Jaeger) | Internal only                          |

## Accessing Observability UIs

All observability UIs are protected by SSO. After authenticating via Keycloak:

1. **Grafana** - Primary entry point for metrics, logs, and alerts
2. **Jaeger** - For deep-dive into request traces
3. **Prometheus** - Direct metric queries (advanced use)

## Key Dashboards

### 1. AI Metrics Dashboard
- **Location**: Grafana → Dashboards → AI Metrics
- **Purpose**: Monitor AI provider performance, rate limits, and costs
- **Key Panels**:
  - Request latency by provider (Groq, OpenRouter, OpenAI)
  - Rate limit hits and cooldown periods
  - Model usage distribution
  - Token consumption

### 2. HTTP Metrics Dashboard
- **Location**: Grafana → Dashboards → HTTP Metrics
- **Purpose**: API endpoint performance
- **Key Panels**:
  - Request rate by endpoint
  - Latency percentiles (p50, p95, p99)
  - Error rate by status code
  - Request size distribution

### 3. Job Queue Metrics Dashboard
- **Location**: Grafana → Dashboards → Job Queue Metrics
- **Purpose**: Evaluation job processing health
- **Key Panels**:
  - Queue depth and processing rate
  - Job success/failure ratio
  - DLQ (Dead Letter Queue) size
  - Worker concurrency utilization

### 4. Request Drilldown Dashboard
- **Location**: Grafana → Dashboards → Request Drilldown
- **Purpose**: Correlate metrics, logs, and traces for specific requests
- **Usage**:
  1. Find request ID from logs or traces
  2. Filter dashboard by `request_id`
  3. View correlated metrics, logs, and trace links

## Log Exploration (Loki)

### Access
Grafana → Explore → Select "Loki" datasource

### Useful Queries

```logql
# All errors from backend
{service="backend"} |= "error"

# Specific request by ID
{job="app-logs"} | json | request_id="<REQUEST_ID>"

# Rate limit events
{service=~"backend|worker"} |= "rate_limit"

# Job processing failures
{service="worker"} |= "evaluation failed"

# Slow requests (>2s)
{service="backend"} | json | latency > 2000
```

### Log Labels
Promtail extracts these labels from structured JSON logs:
- `level` - Log level (info, warn, error)
- `service` - Application service name
- `env` - Environment (dev, prod)
- `request_id` - Unique request identifier
- `trace_id` - Distributed trace ID
- `method`, `path`, `route` - HTTP request details

## Distributed Tracing (Jaeger)

### Finding Traces

1. **By Service**: Select service (backend, worker) and time range
2. **By Trace ID**: Direct lookup with trace ID from logs
3. **By Operation**: Filter by specific operation name

### Trace Structure
```
HTTP Request (backend)
  └── DB Query
  └── Redis Cache Check
  └── AI Provider Call
      └── Token Counting
      └── Response Parsing
  └── Queue Publish
```

## Alerting

### Alert Channels
- **Email**: Configured via Grafana SMTP settings
- **Slack/PagerDuty**: Add via Grafana → Alerting → Contact Points

### Key Alerts (Recommended)

| Alert | Condition | Severity |
|-------|-----------|----------|
| High Error Rate | >5% 5xx in 5min | Critical |
| Queue Backlog | Depth >100 for 10min | Warning |
| AI Provider Unavailable | All providers rate-limited | Critical |
| High Latency | p95 >10s for 5min | Warning |
| DLQ Growing | >10 messages in DLQ | Warning |

### Alert Rules Location
- Prometheus rules: `deploy/prometheus-rules.yml`
- Grafana alerts: `deploy/grafana/provisioning/alerting/`

## Troubleshooting Playbooks

### High Error Rate

1. Check Grafana HTTP Metrics for affected endpoints
2. Query Loki for recent errors:
   ```logql
   {service="backend"} |= "error" | json | line_format "{{.msg}}"
   ```
3. Check trace for failing requests
4. Verify external dependencies (DB, Redis, AI providers)

### Job Processing Stalled

1. Check Job Queue Metrics dashboard - is queue growing?
2. Check worker logs for errors:
   ```logql
   {service="worker"} |= "error"
   ```
3. Check DLQ size and contents
4. Verify Redpanda health: `rpk cluster health`
5. Check AI provider rate limits

### AI Provider Issues

1. Check AI Metrics dashboard for rate limit hits
2. Query logs for provider errors:
   ```logql
   {service=~"backend|worker"} |= "429" or |= "rate_limit"
   ```
3. Verify API key validity
4. Check provider status pages (OpenRouter, Groq, OpenAI)

### Database Performance

1. Check DB connection pool metrics
2. Query slow queries in logs:
   ```logql
   {service="backend"} | json | query_time > 1000
   ```
3. Check PostgreSQL logs via promtail

## Metrics Reference

### Application Metrics (Prometheus)

```promql
# Request rate
rate(http_requests_total[5m])

# Error rate
rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m])

# Latency percentiles
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# AI provider latency
histogram_quantile(0.95, rate(ai_request_duration_seconds_bucket[5m]))

# Queue depth
evaluation_queue_depth

# Active workers
evaluation_workers_active
```

## Maintenance

### Log Retention
- Loki default: 744h (31 days)
- Adjust in `deploy/loki-config.yml` → `limits_config.retention_period`

### Metric Retention
- Prometheus default: 15 days
- Adjust in `docker-compose.prod.yml` → prometheus command `--storage.tsdb.retention.time`

### Dashboard Backups
- Dashboards are provisioned from `deploy/grafana/dashboards/`
- Changes made in UI are not persisted - update JSON files

## Contact

For observability issues or dashboard requests, contact the platform team or create an issue in the repository.
