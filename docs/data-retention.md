# Data Retention Policy

This document defines what data the AI CV Evaluator stores, how long it is retained, and procedures for data management.

## Data Categories

### 1. Uploaded Files (CVs/Resumes)

| Attribute | Value |
|-----------|-------|
| **Storage Location** | PostgreSQL (binary/text) |
| **Retention Period** | `DATA_RETENTION_DAYS` (default: 90 days) |
| **Cleanup Mechanism** | Automatic via `CLEANUP_INTERVAL` (default: 24h) |
| **Contains PII** | Yes (names, contact info, work history) |

### 2. Evaluation Results

| Attribute | Value |
|-----------|-------|
| **Storage Location** | PostgreSQL (JSON) |
| **Retention Period** | Same as uploaded files |
| **Cleanup Mechanism** | Cascade delete with parent job |
| **Contains PII** | Derived from CV content |

### 3. Job Metadata

| Attribute | Value |
|-----------|-------|
| **Storage Location** | PostgreSQL |
| **Retention Period** | Same as uploaded files |
| **Data Stored** | Job ID, status, timestamps, source IP |

### 4. Dead Letter Queue (DLQ)

| Attribute | Value |
|-----------|-------|
| **Storage Location** | Redpanda topic `evaluation-dlq` |
| **Retention Period** | `DLQ_MAX_AGE` (default: 7 days) |
| **Cleanup Mechanism** | Topic-level retention |
| **Purpose** | Failed job reprocessing |

### 5. Vector Embeddings (RAG)

| Attribute | Value |
|-----------|-------|
| **Storage Location** | Qdrant vector database |
| **Retention Period** | Indefinite (reference data) |
| **Data Stored** | Job descriptions, scoring rubrics |
| **Contains PII** | No (reference materials only) |

### 6. Logs

| Attribute | Value |
|-----------|-------|
| **Storage Location** | Loki |
| **Retention Period** | 31 days (default) |
| **Contains PII** | May include request metadata |
| **Cleanup Mechanism** | Loki automatic retention |

### 7. Metrics

| Attribute | Value |
|-----------|-------|
| **Storage Location** | Prometheus |
| **Retention Period** | 15 days (default) |
| **Contains PII** | No (aggregated statistics only) |

### 8. Traces

| Attribute | Value |
|-----------|-------|
| **Storage Location** | Jaeger (in-memory/Badger) |
| **Retention Period** | 7 days (default) |
| **Contains PII** | Request IDs, may include paths |

## Configuration

### Environment Variables

```bash
# PostgreSQL data retention
DATA_RETENTION_DAYS=90        # Days to keep uploaded files and results
CLEANUP_INTERVAL=24h          # How often cleanup job runs

# DLQ retention
DLQ_MAX_AGE=168h              # 7 days
DLQ_CLEANUP_INTERVAL=1h       # How often DLQ cleanup runs

# Observability (configured in respective config files)
# Loki: deploy/loki-config.yml → limits_config.retention_period
# Prometheus: docker-compose.prod.yml → --storage.tsdb.retention.time
```

### Adjusting Retention Periods

1. **Increase retention** (e.g., for compliance):
   ```bash
   # In .env.production
   DATA_RETENTION_DAYS=365
   ```

2. **Decrease retention** (e.g., for GDPR minimization):
   ```bash
   DATA_RETENTION_DAYS=30
   ```

3. **Disable automatic cleanup** (not recommended):
   ```bash
   CLEANUP_INTERVAL=0
   ```

## Data Deletion Procedures

### User-Requested Deletion (GDPR/CCPA)

For data subject access requests:

1. **Identify records by user identifier**:
   ```sql
   SELECT * FROM jobs WHERE source_ip = 'x.x.x.x' OR email = 'user@example.com';
   ```

2. **Delete specific job and cascaded data**:
   ```sql
   DELETE FROM jobs WHERE id = '<job_id>';
   ```

3. **Remove from DLQ if present**:
   ```bash
   # Use Redpanda Console or rpk to identify and delete messages
   rpk topic consume evaluation-dlq --format json | grep '<job_id>'
   ```

4. **Purge from Loki** (if needed):
   - Loki does not support granular deletion
   - Wait for retention period or contact ops for log rotation

### Bulk Cleanup

Force immediate cleanup of expired data:

```bash
# Connect to backend container
docker exec -it backend sh

# Trigger manual cleanup (if endpoint exists)
curl -X POST http://localhost:8080/admin/cleanup
```

### Complete Data Wipe (Environment Reset)

```bash
# Stop all services
docker compose -f docker-compose.prod.yml down

# Remove volumes (DESTRUCTIVE)
docker volume rm $(docker volume ls -q | grep ai-cv-evaluator)

# Restart services
docker compose -f docker-compose.prod.yml up -d
```

## Backup and Recovery

### Database Backup

```bash
# Create backup
docker exec db pg_dump -U postgres app > backup_$(date +%Y%m%d).sql

# Restore from backup
docker exec -i db psql -U postgres app < backup_20240101.sql
```

### Recommended Backup Schedule

| Data | Frequency | Retention |
|------|-----------|-----------|
| PostgreSQL | Daily | 30 days |
| Qdrant (if modified) | Weekly | 4 weeks |
| Config files | On change | Version controlled |

### Disaster Recovery

1. **Provision new infrastructure** (same Docker Compose setup)
2. **Restore PostgreSQL from latest backup**
3. **Re-seed RAG data** if Qdrant not backed up:
   ```bash
   make seed-rag
   ```
4. **Verify services**: Check `/healthz` and `/readyz`

## Compliance Considerations

### GDPR (EU)

- **Right to Erasure**: Supported via job deletion
- **Data Minimization**: Configurable retention periods
- **Purpose Limitation**: CVs used only for evaluation

### Data Residency

- All data stored on infrastructure defined in `docker-compose.prod.yml`
- For multi-region, ensure database and storage comply with residency requirements

### Audit Trail

- All API requests logged with request ID
- Job lifecycle events recorded with timestamps
- Access via Loki or Grafana Explore

## Monitoring Data Growth

### Key Metrics

```promql
# PostgreSQL database size
pg_database_size_bytes{datname="app"}

# Qdrant collection size
qdrant_collection_vectors_count

# Loki ingestion rate
loki_distributor_bytes_received_total

# Redpanda topic size
redpanda_kafka_log_size_bytes
```

### Alerts

Configure alerts for:
- Database size >80% of disk
- Rapid growth in log volume
- DLQ growing unexpectedly

## Contact

For data retention policy questions or deletion requests, contact the platform team.
