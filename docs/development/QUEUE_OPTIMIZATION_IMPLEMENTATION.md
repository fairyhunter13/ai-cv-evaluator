# Queue Optimization & Retry Implementation

## 🎯 **Overview**

This document describes the implementation of queue optimization with multiple partitions for parallel processing and retry/DLQ mechanisms in the AI CV Evaluator system. These optimizations significantly improve E2E test performance and system reliability.

## 🚀 **Key Optimizations Implemented**

### **1. Multiple Queue Partitions**
- **Partitions**: 8 partitions (vs 1 previously)
- **Benefits**: True parallel processing of jobs
- **Impact**: 8x potential throughput improvement

### **2. Optimized Topic Configuration**
- **Compression**: Snappy compression for faster processing
- **Retention**: 7-day retention with optimized cleanup
- **Segments**: 1-hour segments for better performance
- **Replication**: Single replica for development/testing

### **3. Multiple Worker Instances**
- **Workers**: 4 separate worker instances
- **Partition Assignment**: Each worker handles 2 partitions
- **Load Balancing**: Automatic partition assignment
- **Fault Tolerance**: Independent worker failures

### **4. Enhanced Consumer Configuration**
- **Fetch Size**: 1MB fetch size for better throughput
- **Fetch Wait**: 100ms optimized wait time
- **Max Records**: 1000 records per fetch
- **Partition Bytes**: 1MB per partition limit

### **5. Retry and DLQ Mechanisms**
- **Automatic Retry**: Exponential backoff with configurable retries
- **Dead Letter Queue**: Failed jobs moved to DLQ for analysis
- **Error Classification**: Smart retry decisions based on error types
- **Reprocessing**: DLQ jobs can be reprocessed if needed

## 📊 **Architecture Changes**

### **Before (Single Partition)**:
```
┌─────────────────┐
│   Producer      │
│                 │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│  Single Topic   │
│  (1 Partition)  │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│  Single Worker  │
│  (Sequential)   │
└─────────────────┘
```

### **After (Multiple Partitions)**:
```
┌─────────────────┐
│   Producer      │
│                 │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│  Optimized Topic│
│  (8 Partitions) │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│  Worker Pool    │
│  ┌─────────────┐│
│  │ Worker 1    ││ ← Partitions 0,1
│  │ (2 parts)   ││
│  └─────────────┘│
│  ┌─────────────┐│
│  │ Worker 2    ││ ← Partitions 2,3
│  │ (2 parts)   ││
│  └─────────────┘│
│  ┌─────────────┐│
│  │ Worker 3    ││ ← Partitions 4,5
│  │ (2 parts)   ││
│  └─────────────┘│
│  ┌─────────────┐│
│  │ Worker 4    ││ ← Partitions 6,7
│  │ (2 parts)   ││
│  └─────────────┘│
└─────────────────┘
```

## 🔧 **Implementation Details**

### **1. Optimized Topic Creation**

**File**: `internal/adapter/queue/redpanda/topic.go`

```go
func createOptimizedTopicForParallelProcessing(ctx context.Context, client *kgo.Client, topic string, partitions int32, replicationFactor int16) error {
    // Optimized topic configuration for parallel processing
    topicReq.Configs = map[string]*string{
        "cleanup.policy":                    stringPtr("delete"),
        "retention.ms":                      stringPtr("604800000"), // 7 days
        "segment.ms":                        stringPtr("3600000"),   // 1 hour
        "compression.type":                 stringPtr("snappy"),
        "min.insync.replicas":              stringPtr("1"),
        "unclean.leader.election.enable":  stringPtr("false"),
        // ... more optimized settings
    }
}
```

### **2. Enhanced Producer Configuration**

**File**: `internal/adapter/queue/redpanda/producer.go`

```go
// Create optimized topic for parallel processing
partitions := int32(8) // Multiple partitions for parallel processing
replicationFactor := int16(1)

if err := createOptimizedTopicForParallelProcessing(ctx, client, TopicEvaluate, partitions, replicationFactor); err != nil {
    // Fallback to standard topic creation
}
```

### **3. Optimized Consumer Configuration**

**File**: `internal/adapter/queue/redpanda/consumer.go`

```go
// Configure consumer options for parallel processing
opts := []kgo.Opt{
    kgo.SeedBrokers(brokers...),
    kgo.TransactionalID(transactionalID),
    kgo.FetchIsolationLevel(kgo.ReadCommitted()),
    kgo.ConsumerGroup(groupID),
    kgo.ConsumeTopics(topic),
    kgo.RequireStableFetchOffsets(),
    // Optimized settings for parallel processing
    kgo.FetchMaxBytes(1048576),        // 1MB fetch size
    kgo.FetchMaxWait(100*time.Millisecond), // 100ms fetch wait
    kgo.FetchMinBytes(1),              // Minimum bytes to fetch
    kgo.FetchMaxPartitionBytes(1048576), // 1MB per partition
    kgo.FetchMaxRecords(1000),         // Max records per fetch
}
```

### **4. Docker Compose Optimization**

**File**: `docker-compose.e2e-optimized.yml`

```yaml
services:
  # Multiple worker instances for parallel processing
  worker-1:
    environment:
      - CONSUMER_GROUP_ID=ai-cv-evaluator-workers-1
      - CONSUMER_PARTITION_ASSIGNMENT=0,1
      - WORKER_ID=worker-1
      
  worker-2:
    environment:
      - CONSUMER_GROUP_ID=ai-cv-evaluator-workers-2
      - CONSUMER_PARTITION_ASSIGNMENT=2,3
      - WORKER_ID=worker-2
      
  worker-3:
    environment:
      - CONSUMER_GROUP_ID=ai-cv-evaluator-workers-3
      - CONSUMER_PARTITION_ASSIGNMENT=4,5
      - WORKER_ID=worker-3
      
  worker-4:
    environment:
      - CONSUMER_GROUP_ID=ai-cv-evaluator-workers-4
      - CONSUMER_PARTITION_ASSIGNMENT=6,7
      - WORKER_ID=worker-4
```

## 📈 **Performance Improvements**

### **Expected Performance Gains**:

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| **Partitions** | 1 | 8 | 8x |
| **Workers** | 1 | 4 | 4x |
| **Parallel Jobs** | 1 | 8 | 8x |
| **Throughput** | 1 job/sec | 8 jobs/sec | 8x |
| **Test Time** | 97-116s | 30-45s | 60-70% |

### **Queue Performance Metrics**:

- **Message Throughput**: 8x improvement
- **Processing Latency**: 60-70% reduction
- **Resource Utilization**: 4x better worker utilization
- **Fault Tolerance**: Independent worker failures

## 🎯 **Usage Instructions**

### **1. Run Optimized E2E Tests**:

```bash
# Run optimized E2E tests with queue optimization
make ci-e2e-optimized

# Or with custom parameters
make ci-e2e-optimized E2E_PARALLEL=8 E2E_WORKER_REPLICAS=4
```

### **2. Monitor Queue Performance**:

```bash
# Check Redpanda Console
open http://localhost:8080

# Check worker logs
docker compose -f docker-compose.e2e-optimized.yml logs worker-1
docker compose -f docker-compose.e2e-optimized.yml logs worker-2
docker compose -f docker-compose.e2e-optimized.yml logs worker-3
docker compose -f docker-compose.e2e-optimized.yml logs worker-4
```

### **3. Verify Partition Assignment**:

```bash
# Check topic partitions
docker exec -it ai-cv-evaluator-redpanda-1 rpk topic describe evaluate-jobs

# Check consumer groups
docker exec -it ai-cv-evaluator-redpanda-1 rpk group describe ai-cv-evaluator-workers-1
```

## 🔍 **Monitoring and Debugging**

### **Key Metrics to Monitor**:

1. **Partition Assignment**: Ensure each worker gets 2 partitions
2. **Message Distribution**: Verify even distribution across partitions
3. **Worker Utilization**: Monitor worker CPU/memory usage
4. **Queue Depth**: Track message queue depth per partition
5. **Processing Latency**: Monitor job processing times

### **Debugging Commands**:

```bash
# Check partition assignment
docker exec -it ai-cv-evaluator-redpanda-1 rpk topic describe evaluate-jobs

# Monitor consumer groups
docker exec -it ai-cv-evaluator-redpanda-1 rpk group describe ai-cv-evaluator-workers-1

# Check worker logs
docker compose -f docker-compose.e2e-optimized.yml logs worker-1 --tail=50

# Monitor queue metrics
docker exec -it ai-cv-evaluator-redpanda-1 rpk topic consume evaluate-jobs --num=10
```

## 🚨 **Troubleshooting**

### **Common Issues**:

1. **Partition Assignment Issues**:
   - Check consumer group configuration
   - Verify worker environment variables
   - Monitor partition assignment logs

2. **Message Distribution Problems**:
   - Verify producer partition assignment
   - Check topic configuration
   - Monitor message routing

3. **Worker Scaling Issues**:
   - Check Docker Compose configuration
   - Verify worker dependencies
   - Monitor resource usage

### **Performance Tuning**:

1. **Adjust Partition Count**:
   ```yaml
   # In docker-compose.e2e-optimized.yml
   - --kafka-num-partitions=16  # Increase for more parallelism
   ```

2. **Optimize Worker Count**:
   ```yaml
   # Add more workers
   worker-5:
     # ... configuration
     environment:
       - CONSUMER_PARTITION_ASSIGNMENT=8,9
   ```

3. **Tune Consumer Settings**:
   ```go
   // In consumer.go
   kgo.FetchMaxBytes(2097152),        // 2MB fetch size
   kgo.FetchMaxWait(50*time.Millisecond), // 50ms fetch wait
   ```

## 📋 **Configuration Summary**

### **Queue Optimization Settings**:

| Setting | Value | Purpose |
|---------|-------|---------|
| **Partitions** | 8 | Parallel processing |
| **Workers** | 4 | Load distribution |
| **Partitions/Worker** | 2 | Balanced assignment |
| **Fetch Size** | 1MB | Throughput optimization |
| **Fetch Wait** | 100ms | Latency optimization |
| **Compression** | Snappy | Performance optimization |
| **Retention** | 7 days | Storage optimization |

### **Docker Compose Services**:

- **worker-1**: Partitions 0,1
- **worker-2**: Partitions 2,3  
- **worker-3**: Partitions 4,5
- **worker-4**: Partitions 6,7

This implementation provides a solid foundation for high-performance parallel processing in E2E tests, with the potential for 8x throughput improvement and 60-70% reduction in test execution time.
