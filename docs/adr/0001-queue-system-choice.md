# ADR-0001: Queue System Choice

**Date:** 2025-09-28  
**Status:** Accepted  

## Context

The system requires a reliable job queue for handling asynchronous CV evaluation tasks. The evaluation process involves multiple steps (text extraction, AI processing, RAG retrieval) and can take 15-30 seconds to complete. We need a queue system that provides:

- Reliable job delivery and processing
- Retry mechanisms with exponential backoff
- Dead letter queue for failed jobs
- Job status tracking and monitoring
- Easy local development setup

## Decision

We will use **Asynq** (Redis-backed job queue) for our queuing system.

## Consequences

### Positive
- **Redis Integration**: Leverages existing Redis infrastructure used for caching
- **Go Native**: Written in Go with excellent Go SDK and type safety
- **Feature Rich**: Built-in retry logic, exponential backoff, cron jobs, monitoring
- **Local Dev Friendly**: Simple Redis setup, web UI for monitoring
- **Observability**: Excellent metrics and monitoring capabilities
- **Performance**: High throughput with configurable concurrency

### Negative
- **Redis Dependency**: Adds Redis as a critical dependency
- **Memory Usage**: Jobs stored in Redis memory (though with TTL cleanup)
- **Single Point of Failure**: Redis becomes critical for queue operations
- **Learning Curve**: Team needs to learn Asynq-specific patterns

### Risks
- Redis memory exhaustion if job cleanup fails
- Queue blocking if Redis becomes unavailable
- Potential job loss if Redis loses persistence

## Alternatives Considered

### Option A: Database-based Queue (PostgreSQL)
- **Pros**: No additional infrastructure, ACID transactions, persistent storage
- **Cons**: Lower throughput, complex polling logic, no built-in retry mechanisms
- **Rejected**: Would require custom implementation of retry/backoff logic

### Option B: Apache Kafka
- **Pros**: High throughput, distributed, excellent durability
- **Cons**: Complex setup, over-engineered for our scale, high operational overhead
- **Rejected**: Too complex for a 5-day project with current requirements

### Option C: AWS SQS/Google Cloud Tasks
- **Pros**: Fully managed, highly reliable, auto-scaling
- **Cons**: Vendor lock-in, additional costs, requires cloud deployment
- **Rejected**: Project requirements specify VPS deployment, not cloud-native

### Option D: RabbitMQ
- **Pros**: Industry standard, excellent reliability, rich feature set
- **Cons**: Additional infrastructure, complex configuration, Java-centric ecosystem
- **Rejected**: Added complexity without significant benefits over Asynq for Go projects
