---
trigger: always_on
---

Define the domain model (entities, states, invariants) and a consistent error taxonomy that maps cleanly across layers and HTTP.

# Domain Entities & Invariants
- Upload
  - Fields: `id`, `type` (cv|project), `text`, `filename`, `mime`, `size`, `created_at`.
  - Invariants:
    - `type` in {cv, project}.
    - `size` ≤ configured max (default 10MB); `mime` allowlist: .txt, .pdf, .docx.
    - `text` sanitized (no control chars); non-empty after extraction.

- Job
  - Fields: `id`, `status` (queued|processing|completed|failed), `error`, `created_at`, `updated_at`, `cv_id`, `project_id`, optional `idempotency_key`.
  - State machine:
    - queued → processing → completed
    - queued → processing → failed (with `error`)
    - queued → failed (rare, e.g., missing artifacts)
  - Transitions are monotonic; record `updated_at` on every change.

- Result
  - Fields: `job_id`, `cv_match_rate`, `cv_feedback`, `project_score`, `project_feedback`, `overall_summary`, `created_at`.
  - Bounds:
    - `cv_match_rate` ∈ [0.0, 1.0]
    - `project_score` ∈ [1.0, 10.0] (weighted 1–5 → ×2, min 1)
  - Text fields trimmed; max lengths enforced at adapter (HTTP) layer.

# Value Objects & Aggregation
- ID: UUIDv4 or KSUID; treat as opaque identifiers.
- Status: enumerate `queued|processing|completed|failed`.
- Scores (1–5) per-parameter; compute weighted averages:
  - CV: weights 40/25/20/15 → weighted average (1–5) → normalize to [0,1] by dividing by 5 (×0.2) → `cv_match_rate`.
  - Project: weights 30/25/20/15/10 → average (1–5) → normalize ×2 → clamp to [1,10] → `project_score`.
- Store raw per-parameter scores (optional, future) to enable audits/drilldowns.

Note: `cv_match_rate` is stored and returned as a normalized fraction in [0,1] (e.g., 0.82). For UI display as a percentage, multiply by 100.

# Error Taxonomy (Domain Sentinels)
- `ErrInvalidArgument` → bad or missing inputs, invariant violations.
- `ErrNotFound` → missing uploads/jobs/results.
- `ErrConflict` → idempotency conflict or invalid state transition.
- `ErrRateLimited` → local or upstream rate limiting.
- `ErrUpstreamTimeout` → LLM/embeddings/Vector DB timeout.
- `ErrUpstreamRateLimit` → upstream 429.
- `ErrSchemaInvalid` → LLM JSON invalid against schema.
- `ErrInternal` → unexpected condition.

# Mapping to HTTP (Adapter)
- Use unified error envelope `{ "error": { "code": "STRING", "message": "...", "details": {} } }`.
- Map codes:
  - `ErrInvalidArgument` → 400 `INVALID_ARGUMENT`
  - Request too large → 413 `INVALID_ARGUMENT`
  - Unsupported media type → 415 `INVALID_ARGUMENT`
  - `ErrNotFound` → 404 `NOT_FOUND`
  - `ErrConflict` → 409 `CONFLICT`
  - `ErrRateLimited` → 429 `RATE_LIMITED`
  - `ErrUpstreamTimeout` → 503 `UPSTREAM_TIMEOUT`
  - `ErrUpstreamRateLimit` → 503 `UPSTREAM_RATE_LIMIT`
  - `ErrSchemaInvalid` → 503 `SCHEMA_INVALID`
  - Fallback → 500 `INTERNAL`
- Include `X-Request-Id` in all responses; never leak internal traces to clients.

# Idempotency & Deduplication
- Accept `Idempotency-Key` on POST `/evaluate`; bind key → `(cv_id, project_id, job_description_hash, study_case_hash)`.
- Return existing job on duplicate keys within TTL.
- Ensure worker handler is idempotent (safe retries, upserts for `results`).

# Concurrency & Timeouts
- Pass `context.Context` through layers; set deadlines for IO (DB/Redis/Qdrant/LLM).
- Respect cancellation from asynq; avoid goroutine leaks; use worker pools with bounds.

# Observability Contracts
- Log with `slog` JSON; fields: `request_id`, `trace_id`, `span_id`, `job_id`, `cv_id`, `project_id`.
- Metrics:
  - Job gauges: queued/processing; counters: completed/failed.
  - AI histograms by provider/op; token counters if available.
- Tracing via OpenTelemetry across HTTP → queue → worker → AI/Qdrant.

# Definition of Done (Domain)
- Entities implement invariants; invalid states impossible by construction where feasible.
- Error taxonomy is used across layers; adapters translate to HTTP consistently.
- State transitions audited in logs/metrics; retries safe and observable.
