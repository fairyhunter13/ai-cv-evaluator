# Queue System Migration: Asynq to Redpanda

## Overview

This document describes the migration from Redis+Asynq to Redpanda (Kafka-compatible) for the queue system.

## Why Migrate?

### Asynq Limitations

1. **Slow Polling**: Asynq workers poll Redis with significant delays (40-66 seconds between jobs)
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
API Server → Redpanda (Kafka) → Consumer (push-based, immediate)
```

## Implementation Details

### Producer (API Server)

- Uses `franz-go` client library
- Transactional producer with `RequiredAcks(AllISRAcks())`
- Each message is produced within a transaction for exactly-once semantics
- Job ID used as message key for ordering guarantees

### Consumer (Worker)

- Consumer group: `ai-cv-evaluator-workers`
- Topic: `evaluate-jobs`
- Fetch isolation level: `ReadCommitted` (only reads committed transactions)
- Manual offset commits after successful processing
- Exactly-once processing: offset committed only after job completion

### Exactly-Once Guarantees

1. **Producer Side**:
   - Idempotent producer enabled
   - Transactional ID: `ai-cv-evaluator-producer`
   - Each enqueue wrapped in transaction

2. **Consumer Side**:
   - Read committed isolation level
   - Manual offset management
   - Offset committed only after successful job processing
   - Failed jobs don't commit offset (will be reprocessed)

## Configuration

### Environment Variables

- `KAFKA_BROKERS`: Comma-separated list of Redpanda brokers (e.g., `redpanda:9092`)
- Removed: `REDIS_URL` (no longer needed)

### Docker Compose

```yaml
redpanda:
  image: docker.redpanda.com/redpandadata/redpanda:v24.3.1
  ports:
    - "19092:19092"  # Kafka API (external)
    - "18081:18081"  # Schema Registry
    - "18082:18082"  # Pandaproxy
    - "9644:9644"    # Admin API

redpanda-console:
  image: docker.redpanda.com/redpandadata/console:v2.7.2
  ports:
    - "8090:8080"    # Web UI
```

## Monitoring

### Redpanda Console

Access at: `http://localhost:8090`

Features:
- View topics and partitions
- Monitor consumer groups and lag
- Inspect messages
- View cluster health
- Schema registry management

### Metrics

Redpanda exposes Prometheus metrics at `:9644/metrics`

Key metrics:
- `redpanda_kafka_request_latency_seconds`
- `redpanda_kafka_consumer_group_committed_offset`
- `redpanda_kafka_consumer_group_lag`

## Migration Steps

1. ✅ Add Redpanda and Redpanda Console to docker-compose
2. ✅ Create producer adapter (`internal/adapter/queue/redpanda/producer.go`)
3. ✅ Create consumer adapter (`internal/adapter/queue/redpanda/consumer.go`)
4. ⏳ Update `cmd/server/main.go` to use Redpanda producer
5. ⏳ Update `cmd/worker/main.go` to use Redpanda consumer
6. ⏳ Update configuration and environment handling
7. ⏳ Update unit tests
8. ⏳ Update E2E tests
9. ⏳ Update CI/CD workflows
10. ⏳ Remove Asynq dependencies

## Testing

### Local Testing

```bash
# Start services
docker compose up -d

# Check Redpanda health
docker compose exec redpanda rpk cluster health

# View topics
docker compose exec redpanda rpk topic list

# Monitor consumer group
docker compose exec redpanda rpk group describe ai-cv-evaluator-workers

# Access Redpanda Console
open http://localhost:8090
```

### E2E Tests

E2E tests should now complete much faster:
- Before: 211+ seconds (with timeouts)
- After: ~60-90 seconds (all jobs processed immediately)

## Rollback Plan

If issues arise:

1. Revert docker-compose changes to use Redis
2. Update environment variables back to `REDIS_URL`
3. Revert cmd/server and cmd/worker to use Asynq
4. Redeploy

## Performance Expectations

- **Job Pickup Latency**: <100ms (vs 40-66 seconds with Asynq)
- **Throughput**: 1000+ messages/second per partition
- **Exactly-Once**: Guaranteed via Kafka transactions
- **Consumer Lag**: Near-zero under normal load

## References

- [Redpanda Documentation](https://docs.redpanda.com/)
- [franz-go Client](https://github.com/twmb/franz-go)
- [Kafka Transactions](https://kafka.apache.org/documentation/#semantics)
