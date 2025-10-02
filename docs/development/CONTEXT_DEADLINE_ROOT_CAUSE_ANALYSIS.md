# Context Deadline Exceeded - Root Cause Analysis

## 🔍 **Root Cause Identified**

### **Primary Issue: kgo Library Connection Timeout Configuration**

The "context deadline exceeded" errors are caused by **missing connection timeout configurations** in the kgo Kafka client setup. The kgo library has default connection timeouts that are too short for the Docker environment.

## 📊 **Evidence from Docker Logs**

```
{"time":"2025-10-02T05:29:40.089434794Z","level":"WARN","msg":"context deadline exceeded, using exponential backoff","service":"ai-cv-evaluator","env":"dev","backoff_duration":2000000000,"poll_count":1}
{"time":"2025-10-02T05:30:12.095175989Z","level":"ERROR","msg":"fetch errors detected","service":"ai-cv-evaluator","env":"dev","error_count":1}
{"time":"2025-10-02T05:30:12.095200261Z","level":"ERROR","msg":"fetch error details","service":"ai-cv-evaluator","env":"dev","error_index":0,"error":{"Topic":"","Partition":-1,"Err":{}},"topic":"","partition":-1,"error_type":"context.deadlineExceededError","error_message":"context deadline exceeded"}
```

### **Pattern Analysis**
- **Error Type**: `context.deadlineExceededError`
- **Frequency**: Every ~30 seconds (exponential backoff)
- **Topic**: Empty string (indicates connection-level failure)
- **Partition**: -1 (indicates connection-level failure)

## 🎯 **Root Cause Details**

### **1. Missing kgo Connection Timeout Configuration**

**Current Configuration (Problematic):**
```go
opts := []kgo.Opt{
    kgo.SeedBrokers(brokers...),
    kgo.TransactionalID(transactionalID),
    kgo.FetchIsolationLevel(kgo.ReadCommitted()),
    kgo.ConsumerGroup(groupID),
    kgo.ConsumeTopics(topic),
    kgo.RequireStableFetchOffsets(),
    // ❌ MISSING: Connection timeout configurations
    kgo.FetchMaxBytes(1048576),
    kgo.FetchMaxWait(100 * time.Millisecond),
    kgo.FetchMinBytes(1),
    kgo.FetchMaxPartitionBytes(1048576),
}
```

**Missing Critical Configurations:**
- `kgo.DialTimeout` - Connection establishment timeout
- `kgo.RequestTimeoutOverhead` - Request timeout buffer
- `kgo.RetryTimeout` - Retry timeout for failed requests
- `kgo.SessionTimeout` - Session timeout for consumer groups

### **2. Docker Network Latency**

**Environment Factors:**
- **Docker Bridge Network**: Additional network overhead
- **Container Startup**: Redpanda needs time to fully initialize
- **Service Discovery**: DNS resolution delays in Docker
- **Resource Contention**: Multiple containers competing for resources

## 🔧 **Solution Implementation**

### **1. Add kgo Connection Timeout Configurations**

```go
opts := []kgo.Opt{
    kgo.SeedBrokers(brokers...),
    kgo.TransactionalID(transactionalID),
    kgo.FetchIsolationLevel(kgo.ReadCommitted()),
    kgo.ConsumerGroup(groupID),
    kgo.ConsumeTopics(topic),
    kgo.RequireStableFetchOffsets()),
    
    // ✅ ADD: Connection timeout configurations
    kgo.DialTimeout(30 * time.Second),           // Connection establishment
    kgo.RequestTimeoutOverhead(10 * time.Second), // Request timeout buffer
    kgo.RetryTimeout(60 * time.Second),           // Retry timeout
    kgo.SessionTimeout(30 * time.Second),        // Consumer group session
    
    // Optimized settings for parallel processing
    kgo.FetchMaxBytes(1048576),
    kgo.FetchMaxWait(100 * time.Millisecond),
    kgo.FetchMinBytes(1),
    kgo.FetchMaxPartitionBytes(1048576),
}
```

### **2. Adjust Context Timeout Strategy**

```go
// Increase context timeout to accommodate connection timeouts
fetchCtx, cancel := context.WithTimeout(ctx, 60*time.Second) // Increased from 30s
fetches := c.session.PollFetches(fetchCtx)
```

### **3. Add Connection Health Checks**

```go
// Add connection health check before polling
if err := c.session.Ping(ctx); err != nil {
    slog.Warn("connection health check failed, retrying", slog.Any("error", err))
    time.Sleep(5 * time.Second)
    continue
}
```

## 📈 **Expected Results After Fix**

### **Before Fix:**
- ❌ Context deadline exceeded every 30 seconds
- ❌ Jobs stuck in "processing" status
- ❌ Workers unable to consume messages
- ❌ Exponential backoff with no recovery

### **After Fix:**
- ✅ Stable connection to Redpanda
- ✅ Successful message consumption
- ✅ Jobs processed normally
- ✅ No context deadline errors

## 🎯 **Implementation Priority**

1. **High Priority**: Add kgo connection timeout configurations
2. **Medium Priority**: Adjust context timeout strategy
3. **Low Priority**: Add connection health checks

## 📝 **Testing Strategy**

1. **Unit Tests**: Test connection timeout configurations
2. **Integration Tests**: Test with Docker environment
3. **E2E Tests**: Verify job processing works end-to-end
4. **Load Tests**: Test under concurrent load

## 🔍 **Monitoring and Observability**

### **Metrics to Track:**
- Connection establishment time
- PollFetches success rate
- Context deadline exceeded frequency
- Job processing latency

### **Logs to Monitor:**
- Connection establishment logs
- PollFetches completion logs
- Error recovery logs
- Job processing logs

## 🚀 **Next Steps**

1. **Implement the fix** in `internal/adapter/queue/redpanda/consumer.go`
2. **Test the fix** with Docker environment
3. **Monitor the results** with detailed logging
4. **Verify E2E tests** pass successfully
5. **Document the solution** for future reference
