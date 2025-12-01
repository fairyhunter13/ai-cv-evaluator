# System Architecture

## Overview

The system evaluates a candidate's CV + project report against a job description and study case brief, returning structured results via an HTTP API and admin UI.

High-level components:

- **Server (app)**: HTTP API on port 8080 inside Docker.
- **Worker**: Consumes evaluation jobs from Redpanda and calls AI providers (single worker process by default, with `CONSUMER_MAX_CONCURRENCY` configurable; defaults to `1` for Groq/OpenRouter free tiers, but the dev `docker-compose.yml` uses a higher value to exercise parallelism).
- **Queue**: Redpanda (Kafka-compatible) for async job processing.
- **Vector DB**: Qdrant.
- **Text Extraction**: Apache Tika.
- **Database**: PostgreSQL.
- **Observability**: OTEL collector, Prometheus, Grafana, Jaeger.
- **Admin Frontend**: Vue 3 + Vite app served behind nginx.
- **SSO**: Keycloak + oauth2-proxy.
- **Portal**: Static HTML landing page linking to dashboards and admin UI.

## SSO and Portal as Single Gate

### Identity Provider

- **Keycloak** realm `aicv` (see `deploy/keycloak/realm-aicv*.json`).
- Default dev admin user: `admin` / `admin123`.

### Front Auth (oauth2-proxy)

- oauth2-proxy is configured as an OIDC client of Keycloak.
- Nginx uses `auth_request` to call oauth2-proxy for all protected routes.
- On 401, nginx redirects to `/oauth2/start?rd=...`.

### Single Entry Portal

In both dev and prod:

- All user-facing apps (admin UI, `/v1` API, Grafana, Prometheus, Jaeger, Redpanda) are accessed through **nginx**, not directly.
- Any **unauthenticated** request to one of these paths triggers:
  1. `auth_request` → oauth2-proxy.
  2. If unauthorized, a redirect to SSO login with `rd` fixed to the **portal root** of that host.
  3. After successful login, the user lands on the portal page.
- The portal then provides links to dashboards, which re-use the same SSO session and do not ask for credentials again.

Health endpoints (`/healthz`, `/readyz`) and ACME challenge paths are intentionally left unauthenticated for operational reasons.

## Data Flow (Upload → Evaluate → Result)

1. **Upload** (`POST /v1/upload`):
   - Accepts multipart form with `cv` and `project` files.
   - Uses Tika to extract text and stores uploads in PostgreSQL.

2. **Evaluate** (`POST /v1/evaluate`):
   - Creates a job record in PostgreSQL (`status = queued`).
   - Publishes an `EvaluateTaskPayload` to Redpanda.

3. **Worker**:
   - Consumes messages from Redpanda.
   - Uses `HandleEvaluate` to:
     - Mark job `processing`.
     - Fetch CV + project content.
     - Call the AI client with retries and model fallbacks.
     - Upsert results into the `results` table.
     - Mark job `completed` or `failed`.

4. **Result** (`GET /v1/result/{id}`):
   - Returns queued/processing/failed/completed states.
   - Contains structured `result` object when `completed`.
   - Contains structured error when `failed`.

## AI Providers, RAG, and Rate Limiting

- **AI providers**
  - Groq is used as the primary chat completion provider when `GROQ_API_KEY` / `GROQ_API_KEY_2` are configured.
  - OpenRouter is used as a fallback provider backed by a curated list of free models discovered at runtime.
  - OpenAI is used for embeddings only (RAG) when `OPENAI_API_KEY` is present.

- **LLM chaining & evaluation pipeline**
  - The worker calls `HandleEvaluate`, which delegates to an `IntegratedEvaluationHandler`.
  - The handler:
    - Optionally retrieves additional RAG context from Qdrant collections (`job_description`, `scoring_rubric`) and injects it into prompts.
    - Builds prompts that encode the standardized scoring rubric described in `submissions/project.md`.
    - Uses `ChatJSONWithRetry` to call providers with exponential backoff and model/account fallback.
    - Cleans and validates JSON, clamps numeric ranges, and writes rows in the `results` table with `cv_match_rate`, `cv_feedback`, `project_score`, `project_feedback`, and `overall_summary`.

- **Rate limiting and DLQ cooling**
  - A global Redis+Lua token bucket limiter is warmed from Postgres and updated from provider rate-limit headers (e.g. `retry-after`, `x-ratelimit-*`).
  - Provider-specific blocks temporarily pause Groq or OpenRouter accounts after repeated `429` responses.
  - Retry and backoff are applied both inside the AI client and around the evaluation handler.
  - Persistent failures are sent to a DLQ topic; the DLQ consumer and `RetryManager.ProcessDLQJob` enforce a cooling window for `UPSTREAM_RATE_LIMIT` failures before re-queueing jobs to the main topic.

## Deployment Topology

- **Dev**: `docker-compose.yml` + `make dev-full`.
- **Prod**: `docker-compose.prod.yml` and GitHub Actions `deploy.yml`.
- Public entrypoint is always nginx; backend and worker containers are not exposed directly.
