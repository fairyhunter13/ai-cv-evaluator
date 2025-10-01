# Observability & Monitoring

This document describes the observability and monitoring capabilities of the AI CV Evaluator system.

## Overview

The system implements comprehensive observability through:
- **Circuit Breakers** for fault tolerance
- **Score Drift Detection** for AI model monitoring
- **Metrics Collection** with Prometheus
- **Distributed Tracing** with OpenTelemetry
- **Health Checks** for service dependencies

## Circuit Breaker Pattern

### Purpose
Circuit breakers prevent cascading failures by monitoring service health and temporarily blocking requests to failing services.

### Implementation
Located in `internal/adapter/observability/circuit_breaker.go`

```go
// Circuit breaker states
StateClosed   // Normal operation
StateOpen     // Blocking requests due to failures
StateHalfOpen // Testing if service recovered
```

### Configuration
- **Max Failures**: Number of failures before opening circuit
- **Timeout**: Duration to wait before testing recovery
- **Half-Open Max**: Number of test requests in half-open state

### Usage Example
```go
cb := NewCircuitBreaker("ai-service", 5, 30*time.Second)
err := cb.Call(func() error {
    return aiService.ProcessRequest()
})
```

## Score Drift Detection

### Purpose
Monitors AI model performance to detect when scores deviate from baseline, indicating potential model degradation.

### Implementation
Located in `internal/adapter/observability/score_drift.go`

### Key Features
- **Baseline Tracking**: Establishes reference scores for comparison
- **Sliding Window**: Maintains recent score history
- **Drift Calculation**: Computes deviation from baseline
- **Threshold Monitoring**: Alerts when drift exceeds limits

### Configuration
```go
monitor := NewScoreDriftMonitor(
    "model-v1.0",    // Model version
    "corpus-v1.0",   // Corpus version
    10,              // Window size
    0.15,            // Drift threshold (15%)
)
```

### Usage Example
```go
// Update baseline
monitor.UpdateBaseline("cv_match_rate", 0.85)

// Record new score
monitor.RecordScore("cv_match_rate", 0.78)

// Check for drift
drift := monitor.GetDrift("cv_match_rate")
if drift > threshold {
    // Handle drift alert
}
```

## Metrics Collection

### Prometheus Metrics
The system exposes metrics for:

#### Request Metrics
- `http_requests_total` - Total HTTP requests
- `http_request_duration_seconds` - Request duration
- `http_requests_in_flight` - In-flight requests

#### AI Service Metrics
- `ai_token_usage_total` - AI token consumption
- `ai_request_duration_seconds` - AI request duration
- `ai_request_errors_total` - AI request errors

#### RAG Metrics
- `rag_effectiveness` - RAG retrieval effectiveness
- `rag_retrieval_errors_total` - RAG retrieval errors
- `rag_context_size_bytes` - RAG context size

#### Circuit Breaker Metrics
- `circuit_breaker_status` - Circuit breaker state
- `circuit_breaker_failures_total` - Circuit breaker failures

#### Score Drift Metrics
- `score_drift_detector` - Score drift measurements
- `baseline_score` - Baseline score values

### Metrics Endpoints
- `/metrics` - Prometheus metrics endpoint
- `/healthz` - Health check endpoint
- `/readyz` - Readiness check endpoint

## Health Checks

### Service Dependencies
The system monitors:
- **Database** - PostgreSQL connection
- **Qdrant** - Vector database connection
- **Tika** - Document processing service

### Health Check Endpoints
```bash
# Basic health check
GET /healthz

# Detailed readiness check
GET /readyz
```

### Readiness Check Response
```json
{
  "checks": [
    {
      "name": "db",
      "ok": true
    },
    {
      "name": "qdrant", 
      "ok": true
    },
    {
      "name": "tika",
      "ok": true
    }
  ]
}
```

## Distributed Tracing

### OpenTelemetry Integration
- **Service Name**: `ai-cv-evaluator`
- **Tracing**: Request flow across services
- **Spans**: Individual operation tracking
- **Context Propagation**: Request correlation

### Trace Configuration
```yaml
OTEL_EXPORTER_OTLP_ENDPOINT: "http://jaeger:14268/api/traces"
OTEL_SERVICE_NAME: "ai-cv-evaluator"
```

## Monitoring Best Practices

### 1. Circuit Breaker Configuration
- Set appropriate failure thresholds
- Configure reasonable timeout values
- Monitor circuit breaker state changes

### 2. Score Drift Monitoring
- Establish meaningful baselines
- Set appropriate drift thresholds
- Monitor multiple metric types

### 3. Alerting Rules
```yaml
# Example Prometheus alerting rules
groups:
- name: ai-cv-evaluator
  rules:
  - alert: HighErrorRate
    expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High error rate detected"
      
  - alert: CircuitBreakerOpen
    expr: circuit_breaker_status == 1
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Circuit breaker is open"
      
  - alert: ScoreDriftDetected
    expr: score_drift_detector > 0.2
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "Score drift detected"
```

## Dashboard Configuration

### Grafana Dashboards
Recommended dashboard panels:

1. **Request Metrics**
   - Request rate
   - Response time
   - Error rate
   - Status code distribution

2. **AI Service Metrics**
   - Token usage
   - Request duration
   - Model performance
   - Error rates

3. **RAG Metrics**
   - Retrieval effectiveness
   - Context size
   - Error rates

4. **Circuit Breaker Status**
   - Circuit states
   - Failure rates
   - Recovery times

5. **Score Drift Monitoring**
   - Baseline scores
   - Current scores
   - Drift measurements

## Troubleshooting

### Common Issues

#### Circuit Breaker Stuck Open
- Check service health
- Verify timeout configuration
- Review failure thresholds

#### Score Drift False Positives
- Adjust drift threshold
- Review baseline establishment
- Check for data quality issues

#### Metrics Not Appearing
- Verify Prometheus configuration
- Check metric endpoint accessibility
- Review metric naming conventions

### Debug Commands
```bash
# Check metrics endpoint
curl http://localhost:8080/metrics

# Check health status
curl http://localhost:8080/healthz

# Check readiness
curl http://localhost:8080/readyz
```

## Configuration Reference

### Environment Variables
```bash
# OpenTelemetry
OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:14268/api/traces
OTEL_SERVICE_NAME=ai-cv-evaluator

# Circuit Breaker
CIRCUIT_BREAKER_MAX_FAILURES=5
CIRCUIT_BREAKER_TIMEOUT=30s

# Score Drift
SCORE_DRIFT_WINDOW_SIZE=10
SCORE_DRIFT_THRESHOLD=0.15
```

### Monitoring Stack
- **Prometheus** - Metrics collection
- **Grafana** - Visualization
- **Jaeger** - Distributed tracing
- **AlertManager** - Alerting

## Performance Considerations

### Metrics Overhead
- Metrics collection adds minimal overhead
- Use sampling for high-volume operations
- Consider metric cardinality

### Circuit Breaker Performance
- Circuit breakers add minimal latency
- State transitions are fast
- Memory usage is minimal

### Score Drift Performance
- Sliding window maintains recent scores
- Calculations are O(1) for most operations
- Memory usage scales with window size

## Security Considerations

### Metrics Security
- Metrics endpoint should be protected
- Avoid exposing sensitive data in metrics
- Use authentication for monitoring endpoints

### Health Check Security
- Health checks should not expose sensitive information
- Use appropriate HTTP status codes
- Implement rate limiting for health endpoints
