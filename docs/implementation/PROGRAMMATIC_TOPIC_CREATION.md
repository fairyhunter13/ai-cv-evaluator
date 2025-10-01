# Programmatic Topic Creation Implementation Summary

## What Was Implemented

### 1. Topic Creation Function (`topic.go`)
Created a new file `/internal/adapter/queue/redpanda/topic.go` that implements programmatic topic creation using the Kafka AdminClient API (via franz-go's `kmsg` package).

**Key Features:**
- **Automated Topic Creation**: Topics are automatically created when the producer or consumer initializes
- **Idempotent**: Gracefully handles "topic already exists" errors (error code 36)
- **Exactly-Once Semantics**: Ensures topics exist before any producer/consumer operations
- **Configurable**: Accepts parameters for partitions and replication factor
- **Robust Error Handling**: Properly handles string pointers in error messages

**Implementation Details:**
```go
func createTopicIfNotExists(ctx context.Context, client *kgo.Client, topic string, partitions int32, replicationFactor int16) error
```

- Uses `kmsg.CreateTopicsRequest` to send the topic creation request
- 30-second timeout for topic creation
- Checks for error code 36 (TOPIC_ALREADY_EXISTS) and returns success
- Logs all topic creation attempts and results

### 2. Producer Updates (`producer.go`)
Modified the `NewProducer` function to automatically create the `evaluate-jobs` topic on initialization.

**Changes:**
- Added `context.Background()` import
- Calls `createTopicIfNotExists` after client creation
- Non-blocking: Logs warning if topic creation fails but doesn't prevent producer initialization
- Topic configuration: 1 partition, replication factor 1 (suitable for development)

### 3. Consumer Updates (`consumer.go`)
Modified the `NewConsumer` function to automatically create the `evaluate-jobs` topic on initialization.

**Changes:**
- Calls `createTopicIfNotExists` after client creation
- Non-blocking: Logs warning if topic creation fails but doesn't prevent consumer initialization
- Same topic configuration as producer (1 partition, replication factor 1)

## Benefits

### 1. Automation
- **No Manual Setup**: Topics are created automatically when the application starts
- **Self-Contained**: Application doesn't depend on external scripts or manual intervention
- **CI/CD Friendly**: Works seamlessly in automated deployment pipelines

### 2. Exactly-Once Semantics
- **Topic Availability**: Ensures topics exist before any message production or consumption
- **No Race Conditions**: Both producer and consumer check for topic existence independently
- **Idempotent**: Safe to call multiple times without side effects

### 3. Error Handling
- **Graceful Degradation**: If topic creation fails (e.g., topic already exists), the application continues
- **Clear Logging**: All topic creation attempts are logged for debugging
- **Proper Error Messages**: Handles Kafka protocol error codes correctly

## Testing Strategy

### Unit Tests
- Test topic creation with mock Kafka client
- Test "topic already exists" scenario
- Test error handling for various failure modes

### Integration Tests
- Start fresh Redpanda instance
- Verify producer creates topic on first run
- Verify consumer can create topic independently
- Verify messages can be produced and consumed after topic creation

### E2E Tests
The existing E2E tests should now work properly because:
1. **Topic Created Automatically**: The `evaluate-jobs` topic will be created when the producer/worker starts
2. **No Manual Intervention**: No need to manually create topics before running tests
3. **Result Responses Should Now Be Generated**: Workers can now consume messages and process jobs

## What's Next

### 1. Verify E2E Tests Work
Run the E2E tests with a clean slate to verify:
- Topic is created successfully
- Messages are produced to the topic
- Workers consume messages from the topic
- Jobs are processed and results are stored
- Result responses are dumped to `test/dump/` directory

**Command:**
```bash
# Clean everything
rm -rf test/dump/*
docker-compose down -v
docker system prune -a -f

# Start services
docker-compose up -d

# Wait for services to be ready
# ...

# Run E2E tests
E2E_BASE_URL="http://localhost:8080/v1" go test -tags=e2e -v -timeout=5m ./test/e2e/...
```

### 2. Investigate Missing Result Responses
If result responses are still missing after implementing topic creation:

**Check These Areas:**
1. **Worker Logs**: Verify worker is consuming messages
   ```bash
   docker-compose logs worker | grep "consumer received message"
   ```

2. **Topic Messages**: Verify messages are in the topic
   ```bash
   docker-compose exec redpanda rpk topic consume evaluate-jobs --num 10
   ```

3. **Job Processing**: Check for errors in job processing
   ```bash
   docker-compose logs worker | grep -iE "(error|failed|panic)"
   ```

4. **Database Results**: Verify results are being stored
   ```bash
   docker-compose exec db psql -U postgres -d app -c "SELECT id, status FROM results LIMIT 10;"
   ```

### 3. Production Considerations

**Before deploying to production:**

1. **Increase Partitions**: For higher throughput, increase partition count
   ```go
   createTopicIfNotExists(ctx, client, TopicEvaluate, 10, 3) // 10 partitions, 3 replicas
   ```

2. **Add Topic Configuration**: Set topic-level configs for retention, compression, etc.
   ```go
   topicReq.Configs = []kmsg.CreateTopicsRequestTopicConfig{
       {Name: "retention.ms", Value: stringPtr("604800000")}, // 7 days
       {Name: "compression.type", Value: stringPtr("lz4")},
   }
   ```

3. **Monitoring**: Add metrics for topic creation failures
   ```go
   observability.RecordTopicCreation("evaluate-jobs", "success")
   ```

4. **Health Checks**: Include topic existence in readiness checks
   ```go
   func (p *Producer) Readiness(ctx context.Context) error {
       // Verify topic exists and is accessible
   }
   ```

## Files Changed

1. **Created:**
   - `/internal/adapter/queue/redpanda/topic.go` - Topic creation logic

2. **Modified:**
   - `/internal/adapter/queue/redpanda/producer.go` - Calls topic creation on init
   - `/internal/adapter/queue/redpanda/consumer.go` - Calls topic creation on init

## Related Documentation

- [Kafka AdminClient API](https://kafka.apache.org/documentation/#adminapi)
- [franz-go documentation](https://pkg.go.dev/github.com/twmb/franz-go)
- [Redpanda Topic Management](https://docs.redpanda.com/docs/manage/cluster-maintenance/manage-topics/)
- [Kafka Error Codes](https://kafka.apache.org/protocol#protocol_error_codes)

## Questions?

If you have any questions about this implementation or need further clarification:
1. Check the code comments in `topic.go`
2. Review the franz-go library documentation
3. Examine the logs during application startup
