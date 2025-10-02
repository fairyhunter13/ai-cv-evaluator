# Storage and Queueing Implementation

This document describes the persistence layer, queue system, and data management for the AI CV Evaluator service.

## 🎯 Overview

The storage and queueing system provides:
- **Persistence** for uploads, jobs, and results
- **Background processing** via message queues
- **Vector storage** for RAG functionality
- **Data consistency** and reliability

## 🗄️ Database Schema

### PostgreSQL Tables

#### Uploads Table
```sql
CREATE TABLE uploads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(10) NOT NULL CHECK (type IN ('cv', 'project')),
    text TEXT NOT NULL,
    filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_uploads_type ON uploads(type);
CREATE INDEX idx_uploads_created_at ON uploads(created_at);
```

#### Jobs Table
```sql
CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status VARCHAR(20) NOT NULL CHECK (status IN ('queued', 'processing', 'completed', 'failed')),
    error TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    cv_id UUID NOT NULL REFERENCES uploads(id),
    project_id UUID NOT NULL REFERENCES uploads(id),
    idempotency_key VARCHAR(255) UNIQUE,
    job_description_hash VARCHAR(64),
    study_case_hash VARCHAR(64)
);

CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_created_at ON jobs(created_at);
CREATE INDEX idx_jobs_idempotency_key ON jobs(idempotency_key);
```

#### Results Table
```sql
CREATE TABLE results (
    job_id UUID PRIMARY KEY REFERENCES jobs(id),
    cv_match_rate DECIMAL(3,2) NOT NULL CHECK (cv_match_rate >= 0.0 AND cv_match_rate <= 1.0),
    cv_feedback TEXT NOT NULL,
    project_score DECIMAL(3,1) NOT NULL CHECK (project_score >= 1.0 AND project_score <= 10.0),
    project_feedback TEXT NOT NULL,
    overall_summary TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### Database Migrations

#### Migration System
- **Tool**: Containerized `goose` for migrations
- **Location**: `deploy/migrations/`
- **Naming**: `YYYYMMDD_HHMMSS_description.sql`
- **Execution**: Automatic via Docker Compose dependencies

#### Migration Example
```sql
-- +goose Up
CREATE TABLE uploads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(10) NOT NULL CHECK (type IN ('cv', 'project')),
    text TEXT NOT NULL,
    filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- +goose Down
DROP TABLE uploads;
```

## 🔄 Queue System (Redpanda)

### Queue Configuration
- **Broker**: Redpanda (Kafka-compatible)
- **Topic**: `evaluate_job`
- **Partitions**: 3 (configurable)
- **Replication**: 1 (single node)
- **Retention**: 7 days

### Message Schema
```json
{
  "job_id": "uuid-string",
  "cv_id": "uuid-string", 
  "project_id": "uuid-string",
  "job_description": "string",
  "study_case_brief": "string",
  "created_at": "2024-01-01T00:00:00Z"
}
```

### Producer Implementation
```go
type JobProducer interface {
    EnqueueJob(ctx context.Context, job *domain.Job) error
}

type redpandaProducer struct {
    producer kafka.Producer
    topic    string
}
```

### Consumer Implementation
```go
type JobConsumer interface {
    Start(ctx context.Context) error
    Stop() error
}

type redpandaConsumer struct {
    consumer kafka.Consumer
    handler  JobHandler
    workers  int
}
```

## 🧠 Vector Database (Qdrant)

### Collections

#### Job Description Collection
```json
{
  "collection_name": "job_description",
  "vector_size": 1536,
  "distance": "Cosine",
  "payload_schema": {
    "title": "string",
    "section": "string", 
    "text": "string",
    "version": "string",
    "ingested_at": "string"
  }
}
```

#### Scoring Rubric Collection
```json
{
  "collection_name": "scoring_rubric",
  "vector_size": 1536,
  "distance": "Cosine", 
  "payload_schema": {
    "parameter": "string",
    "weight": "number",
    "description": "string",
    "version": "string",
    "ingested_at": "string"
  }
}
```

### Vector Operations
```go
type VectorStore interface {
    Upsert(ctx context.Context, collection string, points []Point) error
    Search(ctx context.Context, collection string, vector []float32, limit int) ([]ScoredPoint, error)
    CreateCollection(ctx context.Context, collection string, config CollectionConfig) error
}
```

## 🔄 Data Flow

### Upload Flow
1. **Receive files** via multipart form
2. **Extract text** using Apache Tika
3. **Sanitize content** (strip control chars)
4. **Store in database** with metadata
5. **Return IDs** to client

### Evaluation Flow
1. **Create job** with status "queued"
2. **Enqueue message** to Redpanda
3. **Worker consumes** message
4. **Update status** to "processing"
5. **Run AI pipeline** (RAG + LLM)
6. **Store results** in database
7. **Update status** to "completed"

### Result Retrieval
1. **Query job status** from database
2. **Return current status** and results if completed
3. **Handle polling** with appropriate timeouts

## 🛡️ Data Consistency

### Transaction Management
- **Database transactions** for job status transitions
- **Atomic operations** for result storage
- **Rollback on failure** to maintain consistency
- **Idempotent operations** for safe retries

### State Machine
```
queued → processing → completed
  ↓         ↓
failed ← failed
```

### Error Handling
- **Persist failure reasons** for observability
- **Retry logic** with exponential backoff
- **Dead letter queue** for exhausted retries
- **Graceful degradation** on system failures

## 📊 Performance Optimization

### Database Performance
- **Connection pooling**: `pgxpool` with tuned limits
- **Prepared statements**: For frequent queries
- **Indexes**: On frequently queried columns
- **Query timeouts**: Via context deadlines

### Queue Performance
- **Batching**: Group related messages
- **Compression**: Reduce message size
- **Partitioning**: Distribute load across partitions
- **Consumer groups**: Parallel processing

### Vector Performance
- **Batch operations**: Group vector operations
- **Caching**: Frequently accessed vectors
- **Indexing**: Optimize similarity search
- **Memory management**: Control cache size

## 🔒 Security Considerations

### Data Protection
- **Encryption at rest**: Database and vector store
- **Encryption in transit**: TLS for all connections
- **Access control**: Role-based permissions
- **Audit logging**: Track data access

### Input Validation
- **File type validation**: Allowlist approach
- **Size limits**: Prevent resource exhaustion
- **Content sanitization**: Strip malicious content
- **SQL injection**: Parameterized queries

## 📈 Monitoring and Observability

### Metrics
- **Queue depth**: Monitor backlog
- **Processing time**: Track job duration
- **Error rates**: Success/failure tracking
- **Resource usage**: CPU, memory, disk

### Logging
- **Structured logs**: JSON format
- **Correlation IDs**: Track requests end-to-end
- **Error context**: Include relevant details
- **Performance data**: Timing and resource usage

### Alerting
- **Queue backlog**: High depth alerts
- **Processing failures**: Error rate thresholds
- **Resource exhaustion**: Memory/disk alerts
- **Data consistency**: Validation failures

## 🔄 Data Retention

### Retention Policies
- **Uploads**: Configurable TTL (default 30 days)
- **Jobs**: Retain for audit (90 days)
- **Results**: Long-term storage (1 year)
- **Vectors**: Sync with corpus updates

### Cleanup Procedures
- **Automated cleanup**: Scheduled jobs
- **Archive strategy**: Move old data to cold storage
- **Backup retention**: Multiple backup copies
- **Recovery testing**: Regular restore tests

## 🚀 Scaling Considerations

### Horizontal Scaling
- **Database read replicas**: For read-heavy workloads
- **Queue partitioning**: Distribute load
- **Worker scaling**: Auto-scale based on queue depth
- **Vector sharding**: Distribute vector collections

### Vertical Scaling
- **Resource allocation**: CPU, memory, disk
- **Connection limits**: Database and queue
- **Cache sizing**: In-memory caches
- **Storage capacity**: Database and vector store

## ✅ Definition of Done (Storage & Queueing)

### Implementation Requirements
- **Migration system** working with up/down
- **Queue processes jobs** end-to-end
- **Data layer unit tests** with transaction rollbacks
- **Vector operations** functional
- **Error handling** comprehensive

### Performance Requirements
- **Query performance** within acceptable limits
- **Queue throughput** meets demand
- **Vector search** fast and accurate
- **Resource usage** optimized

### Reliability Requirements
- **Data consistency** maintained
- **Error recovery** automatic
- **Backup/restore** tested
- **Monitoring** comprehensive

This document serves as the comprehensive guide for storage and queueing implementation, ensuring reliable, scalable, and performant data management for the AI CV Evaluator service.
