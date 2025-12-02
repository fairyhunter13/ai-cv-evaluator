# ai-cv-evaluator

Backend service that ingests a candidate CV + project report, evaluates against a job description + study case brief, and returns structured results. Built with Go and Clean Architecture.

## 📚 Documentation

All documentation is organized in the [`docs/`](docs/) directory:

- **[📖 Documentation Index](docs/README.md)** - Complete documentation overview
- **[🚀 Developer Quick Reference](docs/DEVELOPER_QUICK_REFERENCE.md)** - Get started quickly
- **[🏗️ System Architecture](docs/architecture/ARCHITECTURE.md)** - System design and architecture
- **[💻 Frontend Development](docs/development/FRONTEND_DEVELOPMENT.md)** - Frontend development guide
- **[🔄 Migration Status](docs/migration/MIGRATION_SUMMARY.md)** - Current migration status
- **[📁 Directory Structure](docs/DIRECTORY_STRUCTURE.md)** - Project structure overview

- **[🌐 Provider Documentation](docs/providers/README.md)** - External AI provider APIs, models, and rate limits used by this service

## Quick Start

- Prereqs: Docker, Docker Compose, Go 1.24+, Node.js 18+
- Set up env for dev (one of):
  ```bash
  cp .env.sample .env
  # or, if available
  make decrypt-env   # uses secrets/env.sops.yaml -> .env
  ```

### Development Environment
```bash
# Start complete development environment
make dev-full
```
This starts:
- Backend services (PostgreSQL, Redpanda, Qdrant, Tika)
- **Database migrations run automatically** via Docker Compose dependencies
- Backend API server (port 8080)
- Frontend development server with HMR (port 3001)
- Redpanda Console (port 8090)

### Production Deployment
```bash
# Start production environment
docker compose -f docker-compose.prod.yml up -d
```
- **Database migrations run automatically** before app services start
- Backend API server
- Frontend (served by Nginx)
- Worker containers
- All supporting services

### API Endpoints
- `POST /v1/upload` (multipart: `cv`, `project`)
- `POST /v1/evaluate` (JSON)
- `GET /v1/result/{id}`
- `GET /healthz`, `GET /readyz`, `GET /metrics`
- `GET /openapi.yaml`
- Admin API: `POST /admin/token`, `GET /admin/api/status`

## API (Contract-first)
See `api/openapi.yaml` for the complete schema. Examples:

- POST `/v1/evaluate` request
  ```json
  {
    "cv_id": "...",
    "project_id": "...",
    "job_description": "...",
    "study_case_brief": "..."
  }
  ```
- Queued response
  ```json
  { "id": "456", "status": "queued" }
  ```
- Completed response
  ```json
  {
    "id": "456",
    "status": "completed",
    "result": {
      "cv_match_rate": 0.82,
      "cv_feedback": "...",
      "project_score": 7.5,
      "project_feedback": "...",
      "overall_summary": "..."
    }
  }
  ```

## Architecture
- **Split Architecture**: Separate server, worker, and frontend containers for optimal scalability
- **Server Container**: Handles HTTP requests, file uploads, and job creation (API-only)
- **Worker Container**: Single optimized worker tuned for free-tier AI providers
	- **1 concurrent worker by default** (`CONSUMER_MAX_CONCURRENCY=1`) for Groq/OpenRouter free tiers
	- Safe to increase `CONSUMER_MAX_CONCURRENCY` in higher-capacity environments when needed
	- Handles all 8 Kafka partitions with dynamic internal scaling when concurrency > 1
	- Exactly-once processing with auto-commit offsets
	- Push-based delivery for immediate job processing
	- Simplified deployment (single worker vs previous 4-worker setup)
- **Frontend Container**: Vue 3 + Vite admin dashboard with Hot Module Replacement
- **Queue System**: Redpanda (Kafka-compatible) for reliable message delivery
  - 8 partitions for parallel processing within single worker
  - Redpanda Console for monitoring topics, consumer groups, and messages
  - Modern SPA with Tailwind CSS styling
  - API communication with backend via HTTP
  - Development: HMR-enabled dev server (port 3001)
  - Production: Static files served by Nginx
- **Clean Architecture** in `internal/` with ports and adapters:
  - `domain/` entities, errors, ports (Queue, AIClient, TextExtractor)
  - `usecase/` orchestration services
  - `adapter/` http, repo (pgx), queue (redpanda), textextractor (Tika), observability, vector (Qdrant)
- **Async Processing**: Redpanda/Kafka queue with worker processes
- **Text Extraction**: Out-of-process using Apache Tika container
- **Observability**: OpenTelemetry traces + Prometheus metrics

See `docs/README.md` for complete documentation index and `docs/architecture/ARCHITECTURE.md` for detailed diagrams.

## Secrets and SOPS

This repository uses SOPS (with age) to encrypt sensitive files so they can be committed safely.

- Encrypted artifacts (all under `secrets/`):
  - `secrets/env.sops.yaml` – encrypted development environment configuration
  - `secrets/env.production.sops.yaml` – encrypted production environment configuration
  - `secrets/project.md.sops` and `secrets/project.md.enc` – encrypted study case project brief
  - `secrets/rfc/**.sops` – encrypted RFC submission markdowns
  - `secrets/cv/**.sops` – encrypted CV files (optimized + original)
  - `secrets/deploy/keycloak/realm-aicv.json.sops` – encrypted Keycloak realm config (SSO + brute force settings)
- Plaintext counterparts such as `.env`, `.env.production`, `submissions/**` (CVs, RFCs, project.md) and `deploy/keycloak/realm-aicv.json` are **gitignored** and should not be committed.

### Local prerequisites
- Install `sops` and `age`.
- Ensure your age private key exists at `~/.config/sops/age/keys.txt`.
- Your public recipient is printed by:
  ```bash
  age-keygen -y ~/.config/sops/age/keys.txt
  ```

### Decrypt
- Dev/prod env (recommended):
  ```bash
  # From secrets/env.sops.yaml -> .env
  make decrypt-env

  # From secrets/env.production.sops.yaml -> .env.production
  make decrypt-env-production
  ```
- Project brief (for local inspection):
  ```bash
  # From secrets/project.md.enc -> submissions/project.md
  make decrypt-project
  ```

### Edit and re-encrypt
For `secrets/project.md.sops` (binary), decrypt to plaintext, edit, then re-encrypt:
```bash
# decrypt to plaintext, edit it
SOPS_AGE_KEY_FILE="$HOME/.config/sops/age/keys.txt" \
  sops -d --input-type binary --output-type binary secrets/project.md.sops > submissions/project.md

# re-encrypt to .sops using your age key (or use Makefile target for .enc)
SOPS_AGE_KEY_FILE="$HOME/.config/sops/age/keys.txt" \
  sops --encrypt --input-type binary --output-type binary submissions/project.md > secrets/project.md.sops
```

For env files, you can edit the encrypted YAML in place and SOPS will re-encrypt on save:
```bash
sops secrets/env.sops.yaml
# or
sops secrets/env.production.sops.yaml
```

### CI/CD
- Store the age private key in a secret (e.g., `SOPS_AGE_KEY`).
- In CI, write the key to `~/.config/sops/age/keys.txt`, then use Make targets to decrypt:
  ```bash
  make decrypt-env           # for dev/test
  make decrypt-env-production  # for production deploy
  ```
- See the CI rules for full examples in `.github/workflows/ci.yml` and `.github/workflows/deploy.yml`.

## Observability
- Metrics:
  - HTTP: `http_requests_total`, `http_request_duration_seconds`
  - Queue: `jobs_enqueued_total`, `jobs_processing`, `jobs_completed_total`, `jobs_failed_total`
- Evaluation distributions: `evaluation_cv_match_rate` [0..1], `evaluation_project_score` [1..10]
- Traces:
  - HTTP, DB, queue worker spans; export via OTLP (`OTEL_EXPORTER_OTLP_ENDPOINT`).

## RAG Seeding
- Seed files live under `configs/rag/`.
- Supported YAML shapes:
  - `items: ["...", "..."]` (list of strings)
  - `texts: ["...", "..."]` (list of strings)
  - `data: [{text: "...", type: rubric|job|..., section: "...", weight: 0.30}]`
- Metadata is carried to Qdrant payload as `source`, `type`, `section`, `weight` and used for simple re-ranking (by `weight` desc).
- Seed both corpora with:
  ```bash
  make seed-rag  # requires QDRANT_URL (defaults http://localhost:6333); uses configured embeddings (e.g., OPENAI_API_KEY)
  ```

## Admin UI & Dashboards

### Frontend Development (Recommended)
- **Frontend**: http://localhost:3001 (Vue 3 + Vite with HMR)
- **Backend API**: http://localhost:8080
- **Development**: `make dev-full` or `make frontend-dev`

### Traditional Backend-Only Admin
- Enable admin by setting credentials:
  JWT is the default admin auth; use `POST /admin/token` to obtain a token.
- Access: http://localhost:8080/admin/ (login required)

### Observability Dashboards
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (anonymous access enabled via docker-compose for local dev)
- **Jaeger**: http://localhost:16686

## Testing
- Unit tests:
  ```bash
  make test
  ```
- E2E (tagged):
  ```bash
  make test-e2e  # assumes running app stack
  ```

## Deployment (Overview)
- CI validates code, tests, and builds the container.
- Deploy via GitHub Actions to a Docker-capable VPS using SSH keys.
- **Database migrations run automatically** via dedicated migration container.
- App runs behind Docker Compose, with Postgres, Redpanda, Qdrant, and Tika on internal network.

## Configuration
Environment variables (see `.env.sample`):
- Core: `APP_ENV`, `PORT`, `DB_URL`, `KAFKA_BROKERS`
- AI: `OPENROUTER_API_KEY`, `OPENROUTER_API_KEY_2`, `OPENAI_API_KEY`, etc.
- Vector DB: `QDRANT_URL`, `QDRANT_API_KEY`
- Extractor: `TIKA_URL`
- Observability: `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`
- Limits & CORS: `MAX_UPLOAD_MB`, `RATE_LIMIT_PER_MIN`, `CORS_ALLOW_ORIGINS`
	- Queue / AI safety: `CONSUMER_MAX_CONCURRENCY` (defaults to 1), `OPENROUTER_MIN_INTERVAL` (defaults to 5s) for free-tier-friendly throughput
- Frontend: `FRONTEND_SEPARATED` (enables API-only mode)

Notes:
- Groq chat uses an internal curated list of models (for example, `llama-3.1-8b-instant`, `llama-3.3-70b-versatile`). Groq model selection and fallback are automatic and not configurable via environment variables.
- OpenRouter chat uses free models discovered from the OpenRouter API; there is no fixed chat model environment variable.
- Embeddings are performed via OpenAI; set `OPENAI_API_KEY` and `EMBEDDINGS_MODEL` (default `text-embedding-3-small`). If `OPENAI_API_KEY` is not set, embeddings and RAG are skipped.
- E2E tests run against live providers (no stub/mock). Ensure `OPENROUTER_API_KEY` (and `OPENAI_API_KEY` for RAG) are present before running E2E.
- Frontend separation: Set `FRONTEND_SEPARATED=true` to enable API-only backend mode.

