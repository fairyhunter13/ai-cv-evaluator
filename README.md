# ai-cv-evaluator

Backend service that ingests a candidate CV + project report, evaluates against a job description + study case brief, and returns structured results. Built with Go and Clean Architecture.

## 📚 Documentation

All documentation is organized in the [`docs/`](docs/) directory:

- **[📖 Documentation Index](docs/README.md)** - Complete documentation overview
- **[🚀 Developer Quick Reference](docs/DEVELOPER_QUICK_REFERENCE.md)** - Get started quickly
- **[🏗️ System Architecture](docs/architecture/ARCHITECTURE.md)** - System design and architecture
- **[💻 Frontend Development](docs/development/FRONTEND_DEVELOPMENT.md)** - Frontend development guide
- **[🔄 Migration Status](docs/migration/REDPANDA_MIGRATION_STATUS.md)** - Current migration status
- **[📁 Directory Structure](docs/directory-structure.md)** - Project structure overview

## Quick Start

- Prereqs: Docker, Docker Compose, Go 1.24+, Node.js 18+
- Copy and edit env (or decrypt the committed encrypted env):
  ```bash
  cp .env.sample .env
  # or, if available
  sops -d .env.sops.yaml > .env && chmod 600 .env
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
- Admin API: `POST /admin/login`, `POST /admin/logout`, `GET /admin/api/status`

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
- **Worker Container**: Processes AI evaluation tasks with high throughput
  - 3 worker replicas with Kafka consumer groups for load balancing
  - Exactly-once processing with manual offset commits
  - Push-based delivery for immediate job processing (<100ms latency)
- **Frontend Container**: Vue 3 + Vite admin dashboard with Hot Module Replacement
- **Queue System**: Redpanda (Kafka-compatible) for reliable message delivery
  - Replaces Redis+Asynq for better performance and scalability
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

See `docs/README.md` for complete documentation index, `docs/architecture/ARCHITECTURE.md` for detailed diagrams, and `docs/production-split-architecture.md` for production setup.

## Secrets and SOPS

This repository uses SOPS (with age) to encrypt sensitive files so they can be committed safely.

- Encrypted artifacts:
  - `project.md.sops` (encrypted project brief)
  - `.env.sops.yaml` (encrypted development environment configuration)
  - `.env.production.sops.yaml` (encrypted production environment configuration)
- Do NOT commit plaintext files such as `project.md` or `.env`. Both are listed in `.gitignore`.

### Local prerequisites
- Install `sops` and `age`.
- Ensure your age private key exists at `~/.config/sops/age/keys.txt`.
- Your public recipient is printed by:
  ```bash
  age-keygen -y ~/.config/sops/age/keys.txt
  ```

### Decrypt
- Project brief:
  ```bash
  sops -d --input-type binary --output-type binary secrets/project.md.sops > docs/project.md
  ```
- Encrypted env:
  ```bash
  sops -d .env.sops.yaml > .env
  chmod 600 .env
  ```

### Edit and re-encrypt
For `secrets/project.md.sops` (binary), decrypt to plaintext, edit, then re-encrypt:
```bash
# decrypt to plaintext, edit it
sops -d --input-type binary --output-type binary secrets/project.md.sops > docs/project.md

# re-encrypt to .sops using your age key (or use Makefile target for .enc)
SOPS_AGE_KEY_FILE="$HOME/.config/sops/age/keys.txt" \
  sops --encrypt --input-type binary --output-type binary docs/project.md > secrets/project.md.sops
```

For `.env.sops.yaml` / `.env.production.sops.yaml` (YAML), you can edit in place and SOPS will re-encrypt on save:
```bash
sops .env.sops.yaml
# or
sops .env.production.sops.yaml
```

### CI/CD
- Store the age private key in a secret (e.g., `SOPS_AGE_KEY`).
- In CI, write the key to `~/.config/sops/age/keys.txt`, then decrypt:
  ```bash
  sops -d .env.sops.yaml > .env
  chmod 600 .env
  ```
- See the CI rules for a full example.

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
  ```bash
  ADMIN_USERNAME=admin ADMIN_PASSWORD=changeme ADMIN_SESSION_SECRET=dev-secret \
  go run ./cmd/server
  ```
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
- AI: `OPENROUTER_API_KEY`, `OPENAI_API_KEY`, etc.
- Vector DB: `QDRANT_URL`, `QDRANT_API_KEY`
- Extractor: `TIKA_URL`
- Observability: `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`
- Limits & CORS: `MAX_UPLOAD_MB`, `RATE_LIMIT_PER_MIN`, `CORS_ALLOW_ORIGINS`
- Frontend: `FRONTEND_SEPARATED` (enables API-only mode)

Notes:
- Chat model default is `openrouter/auto` when `CHAT_MODEL` is unset.
- Embeddings are performed via OpenAI; set `OPENAI_API_KEY` and `EMBEDDINGS_MODEL` (default `text-embedding-3-small`). If `OPENAI_API_KEY` is not set, embeddings and RAG are skipped.
- E2E tests run against live providers (no stub/mock). Ensure `OPENROUTER_API_KEY` (and `OPENAI_API_KEY` for RAG) are present before running E2E.
- Frontend separation: Set `FRONTEND_SEPARATED=true` to enable API-only backend mode.
