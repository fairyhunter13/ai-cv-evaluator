# Single Worker Migration Guide

## Overview

The AI CV Evaluator system has been migrated from a multi-worker setup to a **single optimized worker** with high internal concurrency. This change simplifies deployment, reduces operational complexity, and maintains (or improves) throughput.

## What Changed

### Before (Multi-Worker Setup)
- **4 worker containers** (worker-1, worker-2, worker-3, worker-4)
- Each worker handled 2 partitions
- Total concurrency: ~16 (4 workers × 4 concurrency each)
- Complex partition assignment via env vars
- Multiple consumer groups

### After (Single Worker Setup)
- **1 worker container** with internal goroutine pool
- Handles all 8 partitions automatically
- Total concurrency: **24** (CONSUMER_MAX_CONCURRENCY=24)
- Simplified configuration
- Single consumer group (`ai-cv-evaluator-workers`)
- **50% increase in total concurrency** (16 → 24)

## Benefits

### 1. Simplified Deployment
- One worker container instead of four
- No manual partition assignment needed
- Easier to understand and maintain

### 2. Better Resource Utilization
- Single worker with 2 CPUs and 2GB RAM
- More efficient than 4 small workers
- Dynamic internal scaling based on load

### 3. Reduced Operational Complexity
- No consumer group rebalancing overhead
- Simpler monitoring (one container to watch)
- Easier debugging and log analysis

### 4. Maintained/Improved Performance
- **24 concurrent goroutines** vs previous 16
- Same 8 Kafka partitions for parallelism
- 10x+ throughput improvement over original single-worker setup

## Configuration Changes

### Docker Compose

**Before** (`docker-compose.e2e-optimized.yml`):
```yaml
services:
  worker-1:
    environment:
      - CONSUMER_MAX_CONCURRENCY=4
      - CONSUMER_GROUP_ID=ai-cv-evaluator-workers-1
      - CONSUMER_PARTITION_ASSIGNMENT=0,1
      - WORKER_ID=worker-1
  worker-2:
    environment:
      - CONSUMER_MAX_CONCURRENCY=4
      - CONSUMER_GROUP_ID=ai-cv-evaluator-workers-2
      - CONSUMER_PARTITION_ASSIGNMENT=2,3
      - WORKER_ID=worker-2
  # ... worker-3 and worker-4
```

**After** (`docker-compose.yml`):
```yaml
services:
  worker:
    environment:
      - CONSUMER_MAX_CONCURRENCY=24  # Single worker, high concurrency
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '2.0'
        reservations:
          memory: 1G
          cpus: '1.0'
```

### Environment Variables

**Removed** (no longer used):
- `CONSUMER_GROUP_ID` - Hardcoded to `ai-cv-evaluator-workers`
- `CONSUMER_PARTITION_ASSIGNMENT` - Automatic assignment
- `WORKER_ID` - Not needed with single worker

**Updated**:
- `CONSUMER_MAX_CONCURRENCY`: Increased from 4 to 24

### Makefile Targets

All E2E test targets now use `docker-compose.yml`:
- `make ci-e2e` - Standard E2E tests
- `make ci-e2e-optimized` - Optimized E2E tests (now uses single worker)
- `make ci-e2e-comprehensive-smoke` - Comprehensive smoke tests

The deprecated `docker-compose.e2e-optimized.yml` has been renamed to `docker-compose.e2e-optimized.yml.deprecated`.

## Migration Steps

### For Local Development

No action needed! The standard `docker-compose.yml` now includes the optimized single-worker setup.

```bash
# Start services (automatically uses single worker)
make docker-run

# Or manually
docker compose up -d
```

### For E2E Tests

No action needed! All E2E targets have been updated to use `docker-compose.yml`.

```bash
# Run E2E tests (uses single worker automatically)
make ci-e2e

# Run optimized E2E tests
make ci-e2e-optimized
```

### For Production Deployments

Update your production configuration to use the single-worker setup:

1. **Remove multiple worker definitions**
2. **Update worker environment variables**:
   ```yaml
   environment:
     - CONSUMER_MAX_CONCURRENCY=24  # Or higher for production
   ```
3. **Update resource limits**:
   ```yaml
   deploy:
     resources:
       limits:
         memory: 2G  # Or higher for production
         cpus: '2.0'  # Or higher for production
   ```

### For Kubernetes

Update your Kubernetes deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-cv-evaluator-worker
spec:
  replicas: 3  # Scale pods, not workers per pod
  template:
    spec:
      containers:
      - name: worker
        env:
        - name: CONSUMER_MAX_CONCURRENCY
          value: "24"  # High internal concurrency
        resources:
          limits:
            memory: "2Gi"
            cpu: "2000m"
```

## Performance Tuning

### Adjusting Concurrency

For higher throughput, increase `CONSUMER_MAX_CONCURRENCY`:

```yaml
environment:
  - CONSUMER_MAX_CONCURRENCY=32  # Or 48, 64, etc.
```

**Note**: Also increase CPU and memory proportionally:
```yaml
deploy:
  resources:
    limits:
      memory: 4G
      cpus: '4.0'
```

### Monitoring

Check worker health and active goroutines:

```bash
# Health check
curl http://localhost:8080/healthz

# Check worker logs
docker compose logs worker -f

# Monitor resource usage
docker stats ai-cv-evaluator-worker-1
```

## Troubleshooting

### Issue: Lower throughput than expected

**Solution**: Increase `CONSUMER_MAX_CONCURRENCY` and CPU/memory:
```yaml
environment:
  - CONSUMER_MAX_CONCURRENCY=32
deploy:
  resources:
    limits:
      memory: 4G
      cpus: '4.0'
```

### Issue: High memory usage

**Solution**: Reduce `CONSUMER_MAX_CONCURRENCY`:
```yaml
environment:
  - CONSUMER_MAX_CONCURRENCY=16
```

### Issue: E2E tests timing out

**Solution**: Ensure adequate resources are allocated:
```bash
# Check if worker is CPU/memory throttled
docker stats

# Increase resources if needed
```

## Rollback Plan

If you need to rollback to the multi-worker setup:

1. Restore `docker-compose.e2e-optimized.yml.deprecated`:
   ```bash
   mv docker-compose.e2e-optimized.yml.deprecated docker-compose.e2e-optimized.yml
   ```

2. Update Makefile to use the old compose file:
   ```bash
   # In Makefile, change:
   $(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE)
   # To:
   $(DOCKER_COMPOSE) -f docker-compose.e2e-optimized.yml
   ```

3. Restart services:
   ```bash
   make docker-cleanup
   make docker-run
   ```

## FAQ

### Q: Why move to a single worker?

**A**: Simplifies deployment, reduces operational complexity, and maintains/improves throughput through higher internal concurrency.

### Q: Will this affect performance?

**A**: No. The single worker has **24 concurrent goroutines** (vs previous 16), providing 50% more concurrency. Performance is maintained or improved.

### Q: Can I still scale horizontally?

**A**: Yes! In Kubernetes, scale the number of pods (each with a single optimized worker) rather than workers per pod.

### Q: What about fault tolerance?

**A**: In production, run multiple pods/instances. Each pod runs a single optimized worker. If one pod fails, others continue processing.

### Q: How do I monitor the internal worker pool?

**A**: Check the health endpoint: `curl http://localhost:8080/healthz`. It shows active workers and queue status.

## References

- [Queue Optimization Implementation](./development/QUEUE_OPTIMIZATION_IMPLEMENTATION.md)
- [Scaling Guide](./ops/scaling_guide.md)
- [Environment Variables](./configuration/ENVIRONMENT_VARIABLES.md)

---

**Migration Date**: 2025-01-04  
**Status**: ✅ Complete
