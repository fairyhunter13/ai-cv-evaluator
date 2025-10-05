# Observability Implementation

## Overview

This document describes the comprehensive observability system implemented to provide adaptive timeout management and observable metrics for all external connections in the AI CV Evaluator system.

## 🎯 **Implementation Summary**

### **✅ Completed Features**

1. **Adaptive Timeout System** - Dynamic timeout adjustment based on performance
2. **Observable Metrics** - Comprehensive metrics for all external connections
3. **Circuit Breaker Pattern** - Automatic failure detection and recovery
4. **Health Status Monitoring** - Real-time health checks for all components
5. **Performance Optimization** - Timeout values optimized based on system performance

---

## 🏗️ **Architecture Components**

### **1. Adaptive Timeout Manager (`internal/observability/adaptive_timeout.go`)**

#### **Features:**
- **Dynamic Timeout Adjustment**: Automatically adjusts timeouts based on success/failure rates
- **Performance Tracking**: Monitors operation duration and adjusts accordingly
- **Configurable Bounds**: Min/max timeout limits to prevent extreme values
- **Success Factor**: Reduces timeout by 5% on fast operations
- **Failure Factor**: Increases timeout by 5% on failures
- **Timeout Factor**: Increases timeout by 10% on timeouts

#### **Configuration:**
```go
// Base timeout: 60s, Min: 10s, Max: 300s
adaptiveTimeout := NewAdaptiveTimeoutManager(
    60*time.Second,  // Base timeout
    10*time.Second,  // Min timeout
    300*time.Second, // Max timeout
)
```

#### **Usage:**
```go
// Get current adaptive timeout
timeout := adaptiveTimeout.GetTimeout()

// Create context with adaptive timeout
ctx, cancel := adaptiveTimeout.WithTimeout(context.Background())

// Record operation results
adaptiveTimeout.RecordSuccess(duration)
adaptiveTimeout.RecordFailure(err)
adaptiveTimeout.RecordTimeout()
```

### **2. Connection Metrics (`internal/observability/metrics.go`)**

#### **Features:**
- **Comprehensive Tracking**: Total requests, success/failure counts, timeouts
- **Latency Monitoring**: Min, max, average latency tracking
- **Error Classification**: Detailed error type tracking
- **Circuit Breaker Integration**: Automatic circuit breaker state management
- **Health Assessment**: Real-time health status evaluation

#### **Connection Types:**
- `ConnectionTypeDatabase` - PostgreSQL database operations
- `ConnectionTypeQueue` - Redpanda queue operations
- `ConnectionTypeAI` - OpenRouter/OpenAI API calls
- `ConnectionTypeVectorDB` - Qdrant vector database operations
- `ConnectionTypeTika` - Apache Tika document processing
- `ConnectionTypeHTTP` - HTTP request operations

#### **Operation Types:**
- `OperationTypeQuery` - Database queries
- `OperationTypePoll` - Queue polling
- `OperationTypePublish` - Queue publishing
- `OperationTypeConsume` - Queue consumption
- `OperationTypeChat` - AI chat completions
- `OperationTypeEmbed` - AI embeddings
- `OperationTypeSearch` - Vector database searches
- `OperationTypeExtract` - Document text extraction
- `OperationTypeRequest` - HTTP requests

### **3. Observable Client (`internal/observability/observable_client.go`)**

#### **Features:**
- **Unified Interface**: Single interface for all external connections
- **Automatic Metrics**: Built-in metrics collection and reporting
- **Retry Logic**: Configurable retry with exponential backoff
- **Circuit Breaker**: Automatic failure detection and recovery
- **Health Monitoring**: Real-time health status assessment

#### **Usage:**
```go
// Create observable client
client := NewObservableClient(
    ConnectionTypeAI,
    OperationTypeChat,
    "https://api.openrouter.ai",
    60*time.Second,  // Base timeout
    10*time.Second,  // Min timeout
    300*time.Second, // Max timeout
)

// Execute operation with metrics
err := client.ExecuteWithMetrics(ctx, "chat_completion", func(ctx context.Context) error {
    // Your operation here
    return operation(ctx)
})

// Execute with retry
err := client.ExecuteWithRetry(ctx, "chat_completion", operation, 3, 1*time.Second)
```

### **4. Circuit Breaker (`internal/observability/circuit_breaker.go`)**

#### **Features:**
- **Three States**: Closed, Open, Half-Open
- **Configurable Thresholds**: Failure count and success rate thresholds
- **Automatic Recovery**: Timeout-based recovery from open state
- **State Tracking**: Detailed state change monitoring

#### **Configuration:**
```go
circuitBreaker := NewCircuitBreaker(
    5,              // Max failures before opening
    30*time.Second, // Timeout before half-open
    0.5,            // Success threshold (50%)
)
```

---

## 🔧 **Integration Points**

### **1. Queue Consumer (`internal/adapter/queue/redpanda/consumer.go`)**

#### **Integration:**
- **Observable Client**: Queue polling with adaptive timeouts
- **Metrics Collection**: Poll success/failure rates, latency tracking
- **Health Status**: Real-time consumer health assessment
- **Circuit Breaker**: Automatic failure detection for queue operations

#### **Configuration:**
```go
observableClient := observability.NewObservableClient(
    observability.ConnectionTypeQueue,
    observability.OperationTypePoll,
    brokers[0],
    60*time.Second,  // Base timeout
    10*time.Second,  // Min timeout
    300*time.Second, // Max timeout
)
```

#### **Usage:**
```go
// Poll with observable metrics
err := c.observableClient.ExecuteWithMetrics(ctx, "poll_fetches", func(fetchCtx context.Context) error {
    fetches = c.session.PollFetches(fetchCtx)
    return nil
})
```

### **2. AI Client (`internal/adapter/ai/real/client.go`)**

#### **Integration:**
- **Chat Observable Client**: OpenRouter API calls with adaptive timeouts
- **Embed Observable Client**: OpenAI embeddings with adaptive timeouts
- **Metrics Collection**: API success/failure rates, response times
- **Health Status**: Real-time AI service health assessment

#### **Configuration:**
```go
// Chat operations
chatObservableClient := observability.NewObservableClient(
    observability.ConnectionTypeAI,
    observability.OperationTypeChat,
    cfg.OpenRouterBaseURL,
    chatTimeout,     // Base timeout
    chatTimeout/6,   // Min timeout
    chatTimeout*3,   // Max timeout
)

// Embed operations
embedObservableClient := observability.NewObservableClient(
    observability.ConnectionTypeAI,
    observability.OperationTypeEmbed,
    "https://api.openai.com/v1",
    embedTimeout,     // Base timeout
    embedTimeout/3,   // Min timeout
    embedTimeout*2,   // Max timeout
)
```

### **3. HTTP Handlers (`internal/adapter/httpserver/handlers.go`)**

#### **Integration:**
- **Health Check Observable Client**: Health check operations with adaptive timeouts
- **Metrics Collection**: Health check success/failure rates, response times
- **Comprehensive Health Status**: Database, Qdrant, Tika health monitoring

#### **Configuration:**
```go
healthObservableClient := observability.NewObservableClient(
    observability.ConnectionTypeHTTP,
    observability.OperationTypeRequest,
    "health-check",
    5*time.Second,   // Base timeout
    1*time.Second,   // Min timeout
    10*time.Second,  // Max timeout
)
```

#### **Usage:**
```go
// Health check with observable metrics
err := s.healthObservableClient.ExecuteWithMetrics(r.Context(), "health_check", func(ctx context.Context) error {
    // Health check logic
    return nil
})
```

---

## 📊 **Metrics and Monitoring**

### **1. Connection Metrics**

#### **Request Metrics:**
- `total_requests` - Total number of requests
- `success_requests` - Number of successful requests
- `failure_requests` - Number of failed requests
- `timeout_requests` - Number of timeout requests
- `success_rate` - Success rate percentage
- `timeout_rate` - Timeout rate percentage

#### **Latency Metrics:**
- `avg_latency` - Average operation latency
- `min_latency` - Minimum operation latency
- `max_latency` - Maximum operation latency

#### **Circuit Breaker Metrics:**
- `circuit_state` - Current circuit breaker state
- `circuit_failures` - Number of circuit breaker failures
- `circuit_successes` - Number of circuit breaker successes

### **2. Adaptive Timeout Metrics**

#### **Timeout Configuration:**
- `current_timeout` - Current adaptive timeout value
- `base_timeout` - Base timeout configuration
- `min_timeout` - Minimum timeout limit
- `max_timeout` - Maximum timeout limit

#### **Performance Tracking:**
- `success_count` - Number of successful operations
- `failure_count` - Number of failed operations
- `timeout_count` - Number of timeout operations
- `success_rate` - Success rate percentage

### **3. Health Status**

#### **Component Health:**
- `is_healthy` - Overall health status
- `connection_type` - Type of external connection
- `operation_type` - Type of operation
- `endpoint` - Connection endpoint

#### **Timing Information:**
- `first_request` - First request timestamp
- `last_request` - Last request timestamp
- `last_success` - Last success timestamp
- `last_failure` - Last failure timestamp

---

## 🚀 **Performance Benefits**

### **1. Adaptive Timeout Management**
- **Reduced Latency**: Timeouts automatically adjust based on performance
- **Improved Reliability**: Prevents both premature timeouts and infinite hangs
- **Resource Optimization**: Optimal timeout values for different operations

### **2. Circuit Breaker Protection**
- **Failure Isolation**: Prevents cascading failures
- **Automatic Recovery**: Self-healing system behavior
- **Resource Protection**: Prevents resource exhaustion

### **3. Comprehensive Monitoring**
- **Real-time Visibility**: Complete observability into system behavior
- **Performance Tracking**: Detailed metrics for optimization
- **Health Assessment**: Proactive health monitoring

### **4. Operational Excellence**
- **Debugging Support**: Detailed error tracking and classification
- **Performance Tuning**: Data-driven timeout optimization
- **System Reliability**: Enhanced fault tolerance

---

## 🔍 **Usage Examples**

### **1. Health Check Endpoint**

```bash
# Get comprehensive health status
curl http://localhost:8080/health

# Response includes:
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "1.0.0",
  "checks": [
    {
      "name": "database",
      "ok": true,
      "details": "Connection successful"
    },
    {
      "name": "qdrant",
      "ok": true,
      "details": "Vector database accessible"
    },
    {
      "name": "tika",
      "ok": true,
      "details": "Document processing service available"
    }
  ]
}
```

### **2. Metrics Endpoint**

```bash
# Get comprehensive metrics
curl http://localhost:8080/metrics

# Response includes:
{
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "1.0.0",
  "health_check": {
    "connection_type": "http",
    "operation_type": "request",
    "endpoint": "health-check",
    "total_requests": 150,
    "success_requests": 148,
    "failure_requests": 2,
    "success_rate": "98.67%",
    "avg_latency": "45ms",
    "circuit_state": "closed",
    "is_healthy": true
  }
}
```

### **3. Consumer Health Status**

```go
// Get consumer health status
healthStatus := consumer.GetHealthStatus()

// Response includes:
{
  "consumer_type": "redpanda",
  "group_id": "ai-cv-evaluator",
  "topic": "evaluate-jobs",
  "active_workers": 4,
  "min_workers": 2,
  "max_workers": 10,
  "connection_type": "queue",
  "operation_type": "poll",
  "total_requests": 1000,
  "success_requests": 995,
  "failure_requests": 5,
  "success_rate": "99.50%",
  "avg_latency": "12ms",
  "circuit_state": "closed",
  "is_healthy": true
}
```

---

## 🎯 **Best Practices**

### **1. Timeout Configuration**
- **Start Conservative**: Begin with reasonable base timeouts
- **Monitor Performance**: Use metrics to optimize timeout values
- **Set Bounds**: Always configure min/max timeout limits
- **Test Scenarios**: Validate timeout behavior under load

### **2. Circuit Breaker Tuning**
- **Failure Threshold**: Set appropriate failure count limits
- **Recovery Timeout**: Configure reasonable recovery periods
- **Success Threshold**: Set realistic success rate requirements
- **Monitor State Changes**: Track circuit breaker state transitions

### **3. Metrics Monitoring**
- **Regular Review**: Monitor metrics for performance trends
- **Alert Thresholds**: Set up alerts for critical metrics
- **Capacity Planning**: Use metrics for capacity planning
- **Performance Optimization**: Use metrics to identify bottlenecks

### **4. Health Check Integration**
- **Load Balancer**: Integrate with load balancer health checks
- **Kubernetes**: Use for readiness/liveness probes
- **Monitoring**: Connect to monitoring systems
- **Alerting**: Set up alerts for health check failures

---

## 🔧 **Configuration**

### **1. Environment Variables**

```bash
# Adaptive timeout configuration
ADAPTIVE_TIMEOUT_BASE=60s
ADAPTIVE_TIMEOUT_MIN=10s
ADAPTIVE_TIMEOUT_MAX=300s

# Circuit breaker configuration
CIRCUIT_BREAKER_MAX_FAILURES=5
CIRCUIT_BREAKER_TIMEOUT=30s
CIRCUIT_BREAKER_SUCCESS_THRESHOLD=0.5

# Metrics configuration
METRICS_ENABLED=true
METRICS_INTERVAL=30s
```

### **2. Docker Compose Configuration**

```yaml
services:
  app:
    environment:
      - ADAPTIVE_TIMEOUT_BASE=60s
      - ADAPTIVE_TIMEOUT_MIN=10s
      - ADAPTIVE_TIMEOUT_MAX=300s
      - CIRCUIT_BREAKER_MAX_FAILURES=5
      - CIRCUIT_BREAKER_TIMEOUT=30s
```

---

## 📈 **Future Enhancements**

### **1. Advanced Metrics**
- **Prometheus Integration**: Export metrics to Prometheus
- **Grafana Dashboards**: Create comprehensive dashboards
- **Alerting Rules**: Set up automated alerting
- **Historical Analysis**: Long-term performance analysis

### **2. Machine Learning**
- **Predictive Timeouts**: ML-based timeout prediction
- **Anomaly Detection**: Automatic anomaly detection
- **Performance Optimization**: AI-driven optimization
- **Capacity Planning**: Predictive capacity planning

### **3. Advanced Circuit Breakers**
- **Custom Strategies**: Configurable circuit breaker strategies
- **Multi-level Breakers**: Hierarchical circuit breakers
- **Adaptive Thresholds**: Self-tuning circuit breaker thresholds
- **Integration Patterns**: Advanced integration patterns

---

## 🎉 **Conclusion**

The observability implementation provides a comprehensive solution for monitoring and managing external connections in the AI CV Evaluator system. With adaptive timeouts, observable metrics, circuit breakers, and health monitoring, the system now has:

- **Enhanced Reliability**: Automatic failure detection and recovery
- **Improved Performance**: Optimized timeout values based on real performance
- **Complete Visibility**: Comprehensive metrics and health monitoring
- **Operational Excellence**: Proactive monitoring and alerting capabilities

This implementation ensures the system can handle varying loads, network conditions, and external service availability while maintaining optimal performance and reliability.
