# Performance Tuning Guide

This comprehensive guide provides performance optimization strategies for the AI CV Evaluator system across all components.

## Overview

Performance tuning involves optimizing system resources, application code, database queries, queue processing, and AI service interactions to achieve optimal throughput and response times.

## Performance Metrics

### Key Performance Indicators (KPIs)

| Metric | Target | Measurement |
|--------|--------|-------------|
| **Response Time** | < 100ms | API endpoint latency |
| **Throughput** | > 100 req/s | Requests per second |
| **Job Processing** | < 30s | Average job completion time |
| **Queue Latency** | < 1s | Message processing delay |
| **Database Query** | < 10ms | Average query time |
| **Memory Usage** | < 80% | System memory utilization |
| **CPU Usage** | < 70% | System CPU utilization |

### Monitoring Tools

```bash
# Application metrics
curl http://localhost:8080/metrics

# Prometheus metrics
curl http://localhost:9090/api/v1/query?query=up

# Grafana dashboards
open http://localhost:3000

# Redpanda Console
open http://localhost:8090
```

## Application-Level Optimization

### 1. Go Application Tuning

#### Memory Optimization

**Garbage Collection Tuning**:
```bash
# Optimize GC for low latency
export GOGC=100
export GOMEMLIMIT=2GiB

# Run with GC tuning
go run -gcflags="all=-N -l" ./cmd/server
```

**Memory Pool Configuration**:
```go
// internal/config/config.go
type Config struct {
    // Database connection pool
    DBMaxConns        int `env:"DB_MAX_CONNS" envDefault:"25"`
    DBMinConns        int `env:"DB_MIN_CONNS" envDefault:"5"`
    DBMaxConnLifetime time.Duration `env:"DB_MAX_CONN_LIFETIME" envDefault:"1h"`
    
    // HTTP server tuning
    HTTPReadTimeout  time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"30s"`
    HTTPWriteTimeout time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"30s"`
    HTTPIdleTimeout  time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"120s"`
}
```

**Connection Pooling**:
```go
// internal/adapter/repo/postgres/postgres.go
func NewPool(cfg config.Config) (*pgxpool.Pool, error) {
    config, err := pgxpool.ParseConfig(cfg.DBURL)
    if err != nil {
        return nil, err
    }
    
    config.MaxConns = int32(cfg.DBMaxConns)
    config.MinConns = int32(cfg.DBMinConns)
    config.MaxConnLifetime = cfg.DBMaxConnLifetime
    
    return pgxpool.NewWithConfig(context.Background(), config)
}
```

#### Concurrency Optimization

**Worker Pool Configuration**:
```go
// cmd/worker/main.go
func main() {
    cfg := config.Load()
    
    // Optimize worker concurrency
    maxWorkers := runtime.NumCPU() * 4
    if cfg.WorkerConcurrency > 0 {
        maxWorkers = cfg.WorkerConcurrency
    }
    
    // Configure worker pool
    workerPool := make(chan struct{}, maxWorkers)
    for i := 0; i < maxWorkers; i++ {
        workerPool <- struct{}{}
    }
}
```

**Goroutine Management**:
```go
// internal/adapter/queue/redpanda/consumer.go
func (c *Consumer) Start(ctx context.Context) error {
    // Limit concurrent workers
    semaphore := make(chan struct{}, c.maxConcurrency)
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case semaphore <- struct{}{}:
            go func() {
                defer func() { <-semaphore }()
                c.processMessage(ctx)
            }()
        }
    }
}
```

### 2. HTTP Server Optimization

#### Request Handling

**Connection Limits**:
```go
// internal/adapter/httpserver/server.go
func NewServer(cfg config.Config) *http.Server {
    return &http.Server{
        Addr:         fmt.Sprintf(":%d", cfg.Port),
        ReadTimeout:  cfg.HTTPReadTimeout,
        WriteTimeout: cfg.HTTPWriteTimeout,
        IdleTimeout:  cfg.HTTPIdleTimeout,
        Handler:      c.router,
    }
}
```

**Middleware Optimization**:
```go
// internal/adapter/httpserver/middleware.go
func (m *Middleware) RateLimit() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Optimize rate limiting with Redis or in-memory cache
            if !m.rateLimiter.Allow(r.RemoteAddr) {
                http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

#### Response Optimization

**Compression**:
```go
// internal/adapter/httpserver/middleware.go
func (m *Middleware) Compression() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return gziphandler.GzipHandler(next)
    }
}
```

**Caching Headers**:
```go
// internal/adapter/httpserver/middleware.go
func (m *Middleware) Cache() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Set appropriate cache headers
            if r.URL.Path == "/v1/result/" {
                w.Header().Set("Cache-Control", "public, max-age=300")
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

## Database Optimization

### 1. PostgreSQL Tuning

#### Connection Configuration

**Connection Pool Settings**:
```bash
# Environment variables
export DB_MAX_CONNS=25
export DB_MIN_CONNS=5
export DB_MAX_CONN_LIFETIME=1h
export DB_MAX_CONN_IDLE_TIME=30m
```

**PostgreSQL Configuration**:
```sql
-- postgresql.conf optimizations
shared_buffers = 256MB
effective_cache_size = 1GB
work_mem = 4MB
maintenance_work_mem = 64MB
checkpoint_completion_target = 0.9
wal_buffers = 16MB
default_statistics_target = 100
random_page_cost = 1.1
effective_io_concurrency = 200
```

#### Query Optimization

**Index Optimization**:
```sql
-- Create optimized indexes
CREATE INDEX CONCURRENTLY idx_jobs_status_created ON jobs(status, created_at);
CREATE INDEX CONCURRENTLY idx_jobs_updated_at ON jobs(updated_at);
CREATE INDEX CONCURRENTLY idx_uploads_type_created ON uploads(type, created_at);
CREATE INDEX CONCURRENTLY idx_results_created_at ON results(created_at);

-- Partial indexes for common queries
CREATE INDEX CONCURRENTLY idx_jobs_queued ON jobs(created_at) WHERE status = 'queued';
CREATE INDEX CONCURRENTLY idx_jobs_processing ON jobs(updated_at) WHERE status = 'processing';
```

**Query Optimization**:
```go
// internal/adapter/repo/postgres/jobs_repo.go
func (r *JobsRepo) List(ctx context.Context, offset, limit int) ([]domain.Job, error) {
    // Use prepared statements for better performance
    query := `
        SELECT id, status, error, created_at, updated_at, cv_id, project_id, idempotency_key
        FROM jobs 
        ORDER BY created_at DESC 
        LIMIT $1 OFFSET $2`
    
    rows, err := r.pool.Query(ctx, query, limit, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    // Use efficient scanning
    var jobs []domain.Job
    for rows.Next() {
        var job domain.Job
        err := rows.Scan(&job.ID, &job.Status, &job.Error, &job.CreatedAt, &job.UpdatedAt, &job.CVID, &job.ProjectID, &job.IdemKey)
        if err != nil {
            return nil, err
        }
        jobs = append(jobs, job)
    }
    
    return jobs, nil
}
```

#### Database Maintenance

**Regular Maintenance**:
```sql
-- Daily maintenance script
VACUUM ANALYZE;
REINDEX DATABASE app;

-- Weekly maintenance
VACUUM FULL;
ANALYZE;
```

**Monitoring Queries**:
```sql
-- Check slow queries
SELECT query, mean_time, calls, total_time 
FROM pg_stat_statements 
ORDER BY mean_time DESC 
LIMIT 10;

-- Check table sizes
SELECT schemaname, tablename, 
       pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables 
WHERE schemaname = 'public' 
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- Check index usage
SELECT schemaname, tablename, indexname, idx_scan, idx_tup_read, idx_tup_fetch
FROM pg_stat_user_indexes 
ORDER BY idx_scan DESC;
```

### 2. Database Connection Optimization

**Connection Pool Tuning**:
```go
// internal/config/config.go
type Config struct {
    // Database connection pool settings
    DBMaxConns        int           `env:"DB_MAX_CONNS" envDefault:"25"`
    DBMinConns        int           `env:"DB_MIN_CONNS" envDefault:"5"`
    DBMaxConnLifetime time.Duration `env:"DB_MAX_CONN_LIFETIME" envDefault:"1h"`
    DBMaxConnIdleTime time.Duration `env:"DB_MAX_CONN_IDLE_TIME" envDefault:"30m"`
}
```

**Connection Health Checks**:
```go
// internal/adapter/repo/postgres/health.go
func (r *Repo) HealthCheck(ctx context.Context) error {
    // Use a simple query for health checks
    var result int
    err := r.pool.QueryRow(ctx, "SELECT 1").Scan(&result)
    return err
}
```

## Queue System Optimization

### 1. Redpanda/Kafka Tuning

#### Producer Optimization

**Batch Configuration**:
```go
// internal/adapter/queue/redpanda/producer.go
func NewProducer(cfg config.Config) (*Producer, error) {
    opts := []kgo.Opt{
        kgo.SeedBrokers(cfg.KafkaBrokers...),
        kgo.TransactionalID("ai-cv-evaluator-producer"),
        kgo.RequiredAcks(kgo.AllISRAcks()),
        
        // Optimize batching
        kgo.ProducerBatchMaxBytes(1000000),    // 1MB batches
        kgo.ProducerBatchMaxMessages(1000),    // 1000 messages per batch
        kgo.ProducerLinger(10 * time.Millisecond), // 10ms linger
        
        // Retry configuration
        kgo.RequestRetries(10),
        kgo.RetryBackoffMs(100, 1000),
    }
    
    return &Producer{client: kgo.NewClient(opts...)}, nil
}
```

**Compression Settings**:
```go
// Enable compression for better throughput
kgo.ProducerCompression(kgo.SnappyCompression()),
```

#### Consumer Optimization

**Consumer Group Configuration**:
```go
// internal/adapter/queue/redpanda/consumer.go
func NewConsumer(cfg config.Config) (*Consumer, error) {
    opts := []kgo.Opt{
        kgo.SeedBrokers(cfg.KafkaBrokers...),
        kgo.ConsumerGroup("ai-cv-evaluator-workers"),
        kgo.ConsumeTopics("evaluate-jobs"),
        kgo.TransactionalID("ai-cv-evaluator-consumer"),
        kgo.FetchIsolationLevel(kgo.ReadCommitted()),
        kgo.DisableAutoCommit(),
        
        // Optimize fetch settings
        kgo.FetchMaxBytes(50 * 1024 * 1024),    // 50MB fetch
        kgo.FetchMaxWait(100 * time.Millisecond), // 100ms wait
        kgo.FetchMinBytes(1024),                // 1KB minimum
        
        // Session timeout
        kgo.SessionTimeout(30 * time.Second),
        kgo.HeartbeatInterval(3 * time.Second),
    }
    
    return &Consumer{client: kgo.NewClient(opts...)}, nil
}
```

#### Topic Configuration

**Topic Optimization**:
```bash
# Create optimized topic
docker-compose exec redpanda rpk topic create evaluate-jobs \
  --partitions 3 \
  --replicas 1 \
  --config retention.ms=604800000 \
  --config segment.ms=3600000 \
  --config compression.type=snappy
```

**Partition Strategy**:
```go
// internal/adapter/queue/redpanda/producer.go
func (p *Producer) EnqueueEvaluate(ctx context.Context, payload domain.EvaluateTaskPayload) (string, error) {
    // Use job ID for consistent partitioning
    partition := hash(payload.JobID) % 3
    
    record := &kgo.Record{
        Topic:     "evaluate-jobs",
        Key:       []byte(payload.JobID),
        Value:     data,
        Partition: int32(partition),
    }
    
    return p.produceRecord(ctx, record)
}
```

### 2. Message Processing Optimization

**Batch Processing**:
```go
// internal/adapter/queue/redpanda/consumer.go
func (c *Consumer) processBatch(ctx context.Context, records []*kgo.Record) error {
    // Process multiple records in batch
    var wg sync.WaitGroup
    semaphore := make(chan struct{}, c.maxConcurrency)
    
    for _, record := range records {
        wg.Add(1)
        go func(r *kgo.Record) {
            defer wg.Done()
            semaphore <- struct{}{}
            defer func() { <-semaphore }()
            
            c.processRecord(ctx, r)
        }(record)
    }
    
    wg.Wait()
    return nil
}
```

## AI Service Optimization

### 1. API Request Optimization

#### Connection Pooling

**HTTP Client Configuration**:
```go
// internal/adapter/ai/real/client.go
func New(cfg config.Config) *Client {
    // Optimize HTTP client
    transport := &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        DisableKeepAlives:   false,
    }
    
    return &Client{
        cfg:     cfg,
        chatHC:  &http.Client{Transport: transport, Timeout: 15 * time.Second},
        embedHC: &http.Client{Transport: transport, Timeout: 15 * time.Second},
    }
}
```

#### Request Batching

**Embedding Batching**:
```go
// internal/adapter/ai/real/client.go
func (c *Client) Embed(ctx domain.Context, texts []string) ([][]float32, error) {
    // Batch embeddings for efficiency
    const batchSize = 100
    var allEmbeddings [][]float32
    
    for i := 0; i < len(texts); i += batchSize {
        end := i + batchSize
        if end > len(texts) {
            end = len(texts)
        }
        
        batch := texts[i:end]
        embeddings, err := c.embedBatch(ctx, batch)
        if err != nil {
            return nil, err
        }
        
        allEmbeddings = append(allEmbeddings, embeddings...)
    }
    
    return allEmbeddings, nil
}
```

#### Caching Strategy

**Embedding Cache**:
```go
// internal/adapter/ai/cache.go
type EmbeddingCache struct {
    cache map[string][]float32
    mutex sync.RWMutex
    maxSize int
}

func (c *EmbeddingCache) Get(text string) ([]float32, bool) {
    c.mutex.RLock()
    defer c.mutex.RUnlock()
    
    embedding, exists := c.cache[text]
    return embedding, exists
}

func (c *EmbeddingCache) Set(text string, embedding []float32) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    
    if len(c.cache) >= c.maxSize {
        // Implement LRU eviction
        c.evictLRU()
    }
    
    c.cache[text] = embedding
}
```

### 2. AI Model Optimization

#### Model Selection

**Free Models Optimization**:
```go
// internal/adapter/ai/freemodels/wrapper.go
func (w *FreeModelWrapper) selectOptimalModel(ctx context.Context) (string, error) {
    models, err := w.freeModelsSvc.GetFreeModels(ctx)
    if err != nil {
        return "", err
    }
    
    // Select model based on performance metrics
    bestModel := models[0]
    for _, model := range models {
        if model.Context > bestModel.Context && w.getModelPerformance(model.ID) > w.getModelPerformance(bestModel.ID) {
            bestModel = model
        }
    }
    
    return bestModel.ID, nil
}
```

#### Prompt Optimization

**Prompt Caching**:
```go
// internal/adapter/ai/real/client.go
type PromptCache struct {
    cache map[string]string
    mutex sync.RWMutex
}

func (c *PromptCache) GetCacheKey(systemPrompt, userPrompt string) string {
    hash := sha256.Sum256([]byte(systemPrompt + userPrompt))
    return hex.EncodeToString(hash[:])
}
```

## Frontend Optimization

### 1. Vue.js Application Tuning

#### Bundle Optimization

**Vite Configuration**:
```typescript
// vite.config.ts
export default defineConfig({
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['vue', 'vue-router', 'pinia'],
          ui: ['@headlessui/vue'],
          utils: ['axios', 'date-fns']
        }
      }
    },
    chunkSizeWarningLimit: 1000
  },
  optimizeDeps: {
    include: ['vue', 'vue-router', 'pinia', 'axios']
  }
})
```

#### Component Optimization

**Lazy Loading**:
```typescript
// main.ts
const routes = [
  {
    path: '/dashboard',
    component: () => import('./views/Dashboard.vue')
  },
  {
    path: '/upload',
    component: () => import('./views/Upload.vue')
  }
]
```

**Virtual Scrolling**:
```vue
<!-- Jobs.vue -->
<template>
  <div class="jobs-list">
    <VirtualList
      :items="jobs"
      :item-height="60"
      :container-height="400"
    >
      <template #default="{ item }">
        <JobItem :job="item" />
      </template>
    </VirtualList>
  </div>
</template>
```

### 2. API Request Optimization

#### Request Batching

**API Client Optimization**:
```typescript
// utils/api.ts
class APIClient {
  private requestQueue: Request[] = []
  private batchTimeout: number = 100
  
  async batchRequest(requests: Request[]): Promise<Response[]> {
    return new Promise((resolve) => {
      this.requestQueue.push(...requests)
      
      setTimeout(() => {
        const batch = this.requestQueue.splice(0)
        this.processBatch(batch).then(resolve)
      }, this.batchTimeout)
    })
  }
}
```

#### Caching Strategy

**Response Caching**:
```typescript
// utils/cache.ts
class ResponseCache {
  private cache = new Map<string, { data: any; timestamp: number }>()
  private ttl = 5 * 60 * 1000 // 5 minutes
  
  get(key: string): any | null {
    const item = this.cache.get(key)
    if (!item) return null
    
    if (Date.now() - item.timestamp > this.ttl) {
      this.cache.delete(key)
      return null
    }
    
    return item.data
  }
  
  set(key: string, data: any): void {
    this.cache.set(key, { data, timestamp: Date.now() })
  }
}
```

## System-Level Optimization

### 1. Docker Optimization

#### Container Resource Limits

**Docker Compose Configuration**:
```yaml
# docker-compose.yml
services:
  app:
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
        reservations:
          cpus: '0.5'
          memory: 512M
  
  worker:
    deploy:
      resources:
        limits:
          cpus: '4.0'
          memory: 4G
        reservations:
          cpus: '1.0'
          memory: 1G
```

#### Container Optimization

**Multi-stage Builds**:
```dockerfile
# Dockerfile.server
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
CMD ["./server"]
```

### 2. System Resource Optimization

#### Memory Management

**System Configuration**:
```bash
# /etc/sysctl.conf
vm.swappiness=10
vm.dirty_ratio=15
vm.dirty_background_ratio=5
vm.vfs_cache_pressure=50
```

#### CPU Optimization

**CPU Affinity**:
```bash
# Set CPU affinity for critical processes
taskset -c 0,1 docker-compose up app
taskset -c 2,3 docker-compose up worker
```

## Monitoring and Alerting

### 1. Performance Metrics

#### Application Metrics

**Custom Metrics**:
```go
// internal/adapter/observability/metrics.go
var (
    JobProcessingDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "job_processing_duration_seconds",
            Help:    "Time spent processing jobs",
            Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
        },
        []string{"job_type", "status"},
    )
    
    QueueDepth = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "queue_depth",
            Help: "Number of messages in queue",
        },
        []string{"topic"},
    )
)
```

#### Database Metrics

**Query Performance**:
```sql
-- Enable query statistics
ALTER SYSTEM SET track_activities = on;
ALTER SYSTEM SET track_counts = on;
ALTER SYSTEM SET track_io_timing = on;
SELECT pg_reload_conf();
```

### 2. Alerting Rules

#### Prometheus Alerts

**Alert Configuration**:
```yaml
# deploy/prometheus/alerts.yml
groups:
- name: ai-cv-evaluator
  rules:
  - alert: HighResponseTime
    expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High response time detected"
      
  - alert: HighQueueDepth
    expr: queue_depth > 1000
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Queue depth is too high"
```

## Performance Testing

### 1. Load Testing

#### Load Test Scripts

**Artillery Configuration**:
```yaml
# artillery.yml
config:
  target: 'http://localhost:8080'
  phases:
    - duration: 60
      arrivalRate: 10
    - duration: 120
      arrivalRate: 50
    - duration: 60
      arrivalRate: 100

scenarios:
  - name: "Upload and Evaluate"
    weight: 100
    flow:
      - post:
          url: "/v1/upload"
          formData:
            cv: "@testdata/cv_01.txt"
            project: "@testdata/project_01.txt"
      - post:
          url: "/v1/evaluate"
          json:
            cv_id: "{{ cv_id }}"
            project_id: "{{ project_id }}"
      - loop:
          - get:
              url: "/v1/result/{{ job_id }}"
          - think: 2
        count: 30
```

#### Performance Benchmarks

**Benchmark Scripts**:
```bash
# Run load tests
artillery run artillery.yml

# Monitor performance
curl http://localhost:8080/metrics | grep -E "(http_requests_total|job_processing_duration)"

# Check queue metrics
curl http://localhost:8090/api/topics
```

### 2. Performance Profiling

#### Go Profiling

**CPU Profiling**:
```bash
# Enable CPU profiling
go run -cpuprofile=cpu.prof ./cmd/server

# Analyze profile
go tool pprof cpu.prof
```

**Memory Profiling**:
```bash
# Enable memory profiling
go run -memprofile=mem.prof ./cmd/server

# Analyze memory usage
go tool pprof mem.prof
```

#### Database Profiling

**Query Analysis**:
```sql
-- Enable query logging
ALTER SYSTEM SET log_statement = 'all';
ALTER SYSTEM SET log_min_duration_statement = 1000;
SELECT pg_reload_conf();

-- Analyze slow queries
SELECT query, mean_time, calls, total_time 
FROM pg_stat_statements 
ORDER BY mean_time DESC;
```

## Best Practices

### 1. Development Practices

#### Code Optimization
- Use connection pooling
- Implement proper caching
- Optimize database queries
- Use batch processing
- Implement circuit breakers

#### Monitoring
- Set up comprehensive metrics
- Implement health checks
- Use distributed tracing
- Monitor resource usage
- Set up alerting

### 2. Deployment Practices

#### Resource Planning
- Right-size containers
- Plan for peak loads
- Implement auto-scaling
- Use load balancing
- Monitor resource usage

#### Maintenance
- Regular performance reviews
- Database maintenance
- Log rotation
- Security updates
- Capacity planning

---

*This performance tuning guide should be updated as new optimization techniques are discovered and implemented.*
