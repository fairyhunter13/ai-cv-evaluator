---
trigger: always_on
---

Implement persistence for uploads, jobs, and results, and a robust queue for background processing.

# Datastore
- Postgres in prod; SQLite allowed for local.
- Tables/entities (example):
  - `uploads` (id, type=cv|project, text, filename, mime, size, created_at)
  - `jobs` (id, status=queued|processing|completed|failed, error, created_at, updated_at, cv_id, project_id)
  - `results` (job_id, cv_match_rate, cv_feedback, project_score, project_feedback, overall_summary, created_at)
- Migrations with containerized `goose` tool; store under `deploy/migrations/`.

# Vector DB (Qdrant)
- Collections: `job_description`, `scoring_rubric`.
- Idempotent creation; schema definition in code; distance metric cosine or dot depending on embedding.
- Deployment policy:
  - Run Qdrant as a container in docker compose (internal network only).
  - App connects via `QDRANT_URL=http://qdrant:6333`.
  - `QDRANT_API_KEY` optional for local/dev; omit when running entirely on internal network.
  - Do not use external vendor for vector DB.

# Queue
- Redpanda (Kafka-compatible) for high-performance message queuing.
  - Task type: `evaluate_job` with payload { job_id, cv_id, project_id }.
  - Retries with backoff; DLQ after max retries.
- Optional in-memory queue with same interface for local/dev.

# Transactions & Consistency
- Use DB transactions for job status transitions and result writes.
- Persist failure reasons for observability.

# Schema & Migrations
- Use containerized migration system with dedicated `Dockerfile.migrate` container.
- Migration files under `deploy/migrations/` with timestamped names and clear up/down.
- Migrations run automatically via Docker Compose service dependencies.
- Application services wait for successful migration completion before starting.

# Connection Pooling & Performance
- Use `pgxpool` with tuned limits: max connections, min idle, and connection lifetime.
- Create indexes for frequent lookups (e.g., `jobs(id,status)`, `results(job_id)`).
- Use query timeouts via context; surface slow query logs with threshold (e.g., > 200ms).
- Prefer prepared statements and batched inserts for large writes.

# Repository Patterns
- Repositories implement small interfaces in `internal/domain`.
- Keep transactions in the usecase layer; pass `Tx` handle to repos when needed.
- Return domain errors (e.g., `ErrNotFound`) and wrap with context at the adapter.

# Data Retention & Backups
- Retain `uploads` for a limited period (configurable TTL) or archive to object storage.
- Periodic cleanup job to purge expired uploads and completed jobs beyond retention window.
- Backup strategy: regular logical backups (e.g., `pg_dump`) and test restores.

# Qdrant Collections & Indexing
- Use separate collections: `job_description`, `scoring_rubric` with consistent vector sizes.
- Store metadata payloads (source, section, weight) and create payload indexes for frequent filters.
- Enable `on_disk` payload storage if collections grow; benchmark latency.
- Idempotent creation on startup; skip if exists.

# Redis/Asynq Configuration
- Use dedicated queues (e.g., `evaluate:default`) and set `max_concurrency` per worker.
- Configure retry policy: exponential backoff with max attempts; dead-letter queue for failures.
- Visibility timeouts shorter than overall job timeout; include jitter.
- Expose metrics: queue depth, processing, completed, failed, retry counts.

# Idempotency & Deduplication
- Enforce unique job per `(cv_id, project_id, job_description_hash, study_case_hash)` when `Idempotency-Key` present.
- Store request hash and return existing job id if duplicate seen within TTL.
- Ensure at-least-once semantics from the queue; make job handler idempotent.

# Definition of Done (Storage & Queueing)
- Migration up/down on fresh DB works.
- Queue processes jobs end-to-end; DLQ visible for exhausted retries.
- Data layer unit tests with transaction rollbacks.
