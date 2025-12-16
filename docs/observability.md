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

All observability UIs are protected by SSO. After authenticating via Authelia:

1. **Grafana** - Primary entry point for metrics, logs, and alerts
2. **Jaeger** - For deep-dive into request traces
3. **Prometheus** - Direct metric queries (advanced use)

## Key Dashboards

### 1. AI Metrics Dashboard
- **Location**: Grafana → Dashboards → AI Metrics
- **Purpose**: Monitor AI provider performance, rate limits, and costs
- **Key Panels**:
  - **Total AI Requests** - Cumulative count of all AI provider API calls
  - **Average AI Latency** - Mean response time for AI provider calls
  - **95th Percentile Latency** - p95 response time for SLA monitoring
  - **Total Tokens Used** - Cumulative token consumption across providers (uses `tiktoken-go` for accurate counting)
  - **AI Request Rate** - Requests per second by provider and operation
  - **AI Request Latency Percentiles** - p95/p99 latency timeseries
  - **Median Project Score** - Average project evaluation scores
  - **Median CV Match Rate** - Average CV-to-job matching scores
  - **Total AI Requests by Provider** - Breakdown by Groq, OpenRouter, OpenAI
  - **AI Provider Request Rate** - Rate of AI provider calls by provider
  - **AI Provider Response Time** - Latency percentiles by provider
  - **Median Latency (p50)** - 50th percentile response time
  - **p95 Latency** - 95th percentile response time
  - **p99 Latency** - 99th percentile response time (worst case for 99% of requests)
  - **Max Observed Latency** - Maximum observed AI request duration
- **Token Counting**: Uses `tiktoken-go` (Go port of OpenAI's tiktoken) for accurate token counting across GPT-4, GPT-3.5, Llama, Mistral, Gemma, Qwen, DeepSeek, and Claude models.
- **Note**: AI metrics are recorded by the worker process when evaluations call AI providers. Metrics will show 0 if no evaluations have been triggered recently.

### 2. HTTP Metrics Dashboard
- **Location**: Grafana → Dashboards → HTTP Metrics
- **Purpose**: API endpoint performance and error tracking
- **Key Panels**:
  - **Request Rate by Route** - Requests per second by API endpoint
  - **Request Distribution by Status** - Pie chart of HTTP status codes
  - **Response Time Percentiles by Route** - p50/p95/p99 latency by endpoint
  - **95th Percentile Response Time** - Gauge for overall p95 latency
  - **Error Rate** - Overall error rate percentage
  - **Error Rate Over Time by Route** - Timeseries showing which routes are causing errors
  - **Top Error Routes** - Table of routes with highest error counts, including status codes
  - **AI Provider Request Rate** - Rate of AI provider calls
  - **AI Provider Response Time** - Latency of AI provider calls

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
