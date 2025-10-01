# Redpanda Migration

This document describes the migration from Redis+Asynq to Redpanda (Kafka-compatible) for the queue system.

## Migration Status: ✅ COMPLETED

The migration from Redis+Asynq to Redpanda has been successfully completed with all components implemented and tested.

## Why Migrate?

### Asynq Limitations
1. **Slow Polling**: Workers poll Redis with significant delays (40-66 seconds between jobs)
2. **No Push Notifications**: Workers must actively poll for new tasks
3. **Limited Scalability**: Redis LIST operations don't scale well for high-throughput scenarios
4. **No Built-in Exactly-Once**: Requires custom implementation for idempotency

### Redpanda Advantages
1. **Push-Based Delivery**: Consumers are notified immediately when messages arrive
2. **Exactly-Once Semantics**: Built-in transactional producer and consumer support
3. **Better Performance**: C++ implementation with Kafka protocol compatibility
4. **Horizontal Scalability**: Native support for partitioning and replication
5. **Redpanda Console**: Built-in web UI for monitoring topics, consumer groups, and messages

## Architecture Changes

### Before (Asynq)
```
API Server → Redis (LIST) → Asynq Worker (polls every ~40s)
```

### After (Redpanda)
```
API Server → Redpanda (Topic) → Consumer (push-based, immediate)
```

## Implementation Details

### 1. Redpanda Producer
- **File**: `internal/adapter/queue/redpanda/producer.go`
- **Features**: Transactional producer with exactly-once semantics
- **Client**: franz-go client library
- **Topic**: `evaluate-jobs`

### 2. Redpanda Consumer
- **File**: `internal/adapter/queue/redpanda/consumer.go`
- **Features**: Consumer group with transactional processing
- **Group**: `ai-cv-evaluator-workers`
- **Isolation**: Read committed level for exactly-once processing

### 3. Configuration Updates
- **File**: `internal/config/config.go`
- **Changes**: Added `KafkaBrokers` field (replaces `RedisURL`)
- **Support**: Comma-separated broker list

### 4. Dependencies
- **Added**: `github.com/twmb/franz-go/pkg/kgo`
- **Removed**: All Asynq and Redis dependencies
- **Updated**: go.mod and vendor directory

## Migration Steps Completed

### ✅ Infrastructure Setup
1. Added Redpanda to docker-compose.yml
2. Configured Redpanda broker on port 19092
3. Set up Redpanda Console on port 8090
4. Updated worker and app dependencies

### ✅ Code Implementation
1. Created Redpanda producer with transactional support
2. Created Redpanda consumer with exactly-once semantics
3. Updated main applications to use Redpanda
4. Removed all Asynq and Redis dependencies

### ✅ Testing and Validation
1. Comprehensive testing of transactional behavior
2. Exactly-once semantics validation
3. Performance testing and optimization
4. Error handling and recovery testing

## Benefits Achieved

### Performance Improvements
- **Immediate Processing**: Push-based delivery eliminates polling delays
- **Better Throughput**: Kafka protocol provides superior performance
- **Horizontal Scaling**: Native support for multiple consumers

### Reliability Improvements
- **Exactly-Once Semantics**: Built-in transactional support
- **Durability**: Message persistence and replication
- **Fault Tolerance**: Automatic failover and recovery

### Monitoring Improvements
- **Redpanda Console**: Built-in web UI for monitoring
- **Metrics**: Comprehensive queue and consumer metrics
- **Debugging**: Better visibility into message flow

## Configuration

### Docker Compose
```yaml
redpanda:
  image: docker.redpanda.com/redpandadata/redpanda:latest
  container_name: redpanda
  command:
    - redpanda
    - start
    - --kafka-addr
    - internal://0.0.0.0:9092,external://0.0.0.0:19092
    - --advertise-kafka-addr
    - internal://redpanda:9092,external://localhost:19092
    - --pandaproxy-addr
    - internal://0.0.0.0:8082,external://0.0.0.0:18082
    - --advertise-pandaproxy-addr
    - internal://redpanda:8082,external://localhost:18082
    - --schema-registry-addr
    - internal://0.0.0.0:8081,external://0.0.0.0:18081
  ports:
    - "19092:19092"
    - "18081:18081"
    - "18082:18082"
    - "8090:8090"
```

### Application Configuration
```go
type Config struct {
    KafkaBrokers string `env:"KAFKA_BROKERS" envDefault:"localhost:19092"`
    // ... other config
}
```

## Monitoring

### Redpanda Console
- **URL**: http://localhost:8090
- **Features**: Topic monitoring, consumer group status, message browsing

### Metrics
- **Producer Metrics**: Message rate, error rate, latency
- **Consumer Metrics**: Consumer lag, processing rate, error rate
- **Topic Metrics**: Message count, partition status, replication

## Troubleshooting

### Common Issues
1. **Connection Issues**: Check broker addresses and ports
2. **Consumer Lag**: Monitor consumer group status
3. **Message Loss**: Verify exactly-once configuration
4. **Performance Issues**: Check partition distribution

### Debugging Commands
```bash
# Check Redpanda status
docker compose logs redpanda

# Monitor topics
docker exec -it redpanda rpk topic list

# Check consumer groups
docker exec -it redpanda rpk group list
```

## Conclusion

The migration to Redpanda has been successfully completed, providing:
- **Immediate performance improvements** with push-based processing
- **Enhanced reliability** with exactly-once semantics
- **Better scalability** with horizontal consumer scaling
- **Improved monitoring** with Redpanda Console
- **Simplified architecture** with fewer moving parts

The system now provides a more robust, scalable, and maintainable queue processing solution.
