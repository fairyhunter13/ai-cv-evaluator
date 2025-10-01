# ADR-0006: Migration from Asynq/Redis to Redpanda

## Status
Accepted

## Context

The current system uses Asynq with Redis as the message queue system for handling background job processing. This architecture has served the system well but presents several limitations:

### Current Architecture Issues:
1. **Redis Dependency**: Heavy reliance on Redis for both caching and queue management
2. **Scalability Concerns**: Redis can become a bottleneck under high load
3. **Complexity**: Managing Redis clusters for high availability adds operational complexity
4. **Resource Usage**: Redis memory usage can be unpredictable with large job queues
5. **Monitoring**: Limited visibility into queue performance and job processing metrics

### Business Requirements:
- **High Throughput**: Need to handle large volumes of CV evaluation jobs
- **Reliability**: Zero data loss for critical evaluation jobs
- **Scalability**: Horizontal scaling capability for peak loads
- **Observability**: Better monitoring and debugging capabilities
- **Performance**: Lower latency for job processing

## Decision

We will migrate from Asynq/Redis to Redpanda for our message queue system.

### Redpanda Benefits:
1. **Kafka Compatibility**: Full Kafka API compatibility with better performance
2. **Simplified Operations**: Single binary deployment, no Zookeeper dependency
3. **Better Performance**: 10x faster than Kafka, lower latency
4. **Cloud-Native**: Built for modern cloud environments
5. **Observability**: Built-in metrics and monitoring capabilities
6. **Resource Efficiency**: Lower memory and CPU usage compared to Redis
7. **Durability**: Built-in replication and persistence
8. **Schema Registry**: Integrated schema management for message validation

### Migration Strategy:
1. **Phase 1**: Deploy Redpanda alongside existing Redis system
2. **Phase 2**: Implement new queue adapters using Redpanda
3. **Phase 3**: Migrate job types one by one
4. **Phase 4**: Decommission Redis queue system
5. **Phase 5**: Optimize and tune Redpanda configuration

## Consequences

### Positive Consequences:
- **Improved Performance**: 10x faster message processing
- **Better Scalability**: Horizontal scaling without Redis bottlenecks
- **Enhanced Reliability**: Built-in replication and durability
- **Simplified Operations**: Single binary deployment
- **Better Monitoring**: Comprehensive metrics and observability
- **Future-Proof**: Kafka ecosystem compatibility
- **Resource Efficiency**: Lower memory and CPU usage

### Negative Consequences:
- **Migration Complexity**: Requires careful planning and execution
- **Learning Curve**: Team needs to learn Redpanda/Kafka concepts
- **Operational Changes**: New deployment and monitoring procedures
- **Temporary Dual System**: Running both systems during migration
- **Testing Overhead**: Comprehensive testing of new queue system

### Risks and Mitigation:
1. **Data Loss Risk**: 
   - Mitigation: Implement comprehensive backup and testing procedures
   - Use Redpanda's built-in replication features

2. **Performance Regression**:
   - Mitigation: Thorough performance testing and benchmarking
   - Gradual migration with rollback capability

3. **Operational Complexity**:
   - Mitigation: Comprehensive documentation and training
   - Phased migration approach

## Implementation Plan

### Phase 1: Infrastructure Setup (Week 1-2)
- [ ] Deploy Redpanda cluster in development environment
- [ ] Configure Redpanda with appropriate settings
- [ ] Set up monitoring and observability tools
- [ ] Create development and testing procedures

### Phase 2: Adapter Development (Week 3-4)
- [ ] Implement Redpanda queue adapter
- [ ] Create migration utilities and tools
- [ ] Implement comprehensive testing suite
- [ ] Performance benchmarking and optimization

### Phase 3: Gradual Migration (Week 5-8)
- [ ] Migrate non-critical job types first
- [ ] Implement dual-write capability
- [ ] Monitor performance and reliability
- [ ] Migrate critical job types
- [ ] Validate data consistency

### Phase 4: Cutover and Optimization (Week 9-10)
- [ ] Complete migration of all job types
- [ ] Decommission Redis queue system
- [ ] Optimize Redpanda configuration
- [ ] Update monitoring and alerting
- [ ] Documentation and training

## Technical Details

### Redpanda Configuration:
```yaml
# Core settings
node_id: 0
rpc_server:
  address: "0.0.0.0"
  port: 33145

kafka_api:
  address: "0.0.0.0"
  port: 9092

admin:
  address: "0.0.0.0"
  port: 9644

# Performance tuning
memory: 2G
smp: 4
# Enable compression
compression: "snappy"
# Enable metrics
metrics_endpoint: "0.0.0.0:9644/metrics"
```

### Queue Adapter Interface:
```go
type RedpandaQueue struct {
    producer *kafka.Producer
    consumer *kafka.Consumer
    topics   map[string]string
}

func (r *RedpandaQueue) Enqueue(job *Job) error
func (r *RedpandaQueue) Dequeue() (*Job, error)
func (r *RedpandaQueue) Process(job *Job) error
```

### Migration Utilities:
- **Job Migration Tool**: Migrate existing jobs from Redis to Redpanda
- **Dual-Write System**: Write to both systems during transition
- **Validation Tools**: Ensure data consistency between systems
- **Rollback Procedures**: Quick rollback to Redis if needed

## Monitoring and Observability

### Key Metrics:
- **Throughput**: Messages per second
- **Latency**: End-to-end processing time
- **Error Rate**: Failed job processing rate
- **Queue Depth**: Number of pending jobs
- **Resource Usage**: CPU, memory, disk usage

### Alerting:
- High error rates (>5%)
- Queue depth exceeding thresholds
- Performance degradation
- Resource usage anomalies

## Rollback Plan

If issues arise during migration:
1. **Immediate**: Switch back to Redis queue system
2. **Data Recovery**: Restore from Redis backup
3. **Investigation**: Analyze root cause of issues
4. **Resolution**: Fix issues before retrying migration
5. **Documentation**: Update procedures based on learnings

## Success Criteria

### Performance Metrics:
- [ ] 10x improvement in message processing speed
- [ ] 50% reduction in resource usage
- [ ] 99.9% job processing success rate
- [ ] Sub-second job processing latency

### Operational Metrics:
- [ ] Zero data loss during migration
- [ ] Successful migration of all job types
- [ ] Team trained on new system
- [ ] Comprehensive documentation updated

## References

- [Redpanda Documentation](https://docs.redpanda.com/)
- [Kafka Protocol Guide](https://kafka.apache.org/protocol)
- [Migration Best Practices](https://docs.redpanda.com/docs/manage/console/migrate/)
- [Performance Tuning Guide](https://docs.redpanda.com/docs/manage/console/performance/)

## Related ADRs

- [ADR-0001: Queue System Choice](./0001-queue-system-choice.md) - Original queue system decision
- [ADR-0002: Vector Database Choice](./0002-vector-database-choice.md) - Related infrastructure decisions
- [ADR-0004: Deployment Strategy](./0004-deployment-strategy.md) - Deployment considerations

## Decision Date
2024-01-15

## Review Date
2024-04-15
