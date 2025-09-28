# ai-cv-evaluator

Backend service that ingests a candidate CV + project report, evaluates against a job description + study case brief, and returns structured results. Built with Go and Clean Architecture.

## Quick Start

- Prereqs: Docker, Docker Compose, Go 1.24+
- Copy and edit env (or decrypt the committed encrypted env):
  ```bash
  cp .env.sample .env
  # or, if available
  sops -d .env.sops.yaml > .env && chmod 600 .env
  ```
- Start local stack:
  ```bash
  docker compose up -d --build
  ```
- Endpoints:
  - `POST /v1/upload` (multipart: `cv`, `project`)
  - `POST /v1/evaluate` (JSON)
  - `GET /v1/result/{id}`
  - `GET /healthz`, `GET /readyz`, `GET /metrics`
  - `GET /openapi.yaml`

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
- Clean Architecture in `internal/` with ports and adapters:
  - `domain/` entities, errors, ports (Queue, AIClient, TextExtractor)
  - `usecase/` orchestration services
  - `adapter/` http, repo (pgx), queue (asynq/redis), textextractor (Tika), observability, vector (Qdrant)
- Async evaluation via Redis/Asynq; worker processes jobs.
- Text extraction is out-of-process using Apache Tika container.
- Observability: OpenTelemetry traces + Prometheus metrics.

See `ARCHITECTURE.md` for diagrams and deeper details.

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
  sops -d --input-type binary --output-type binary project.md.sops > project.md
  ```
- Encrypted env:
  ```bash
  sops -d .env.sops.yaml > .env
  chmod 600 .env
  ```

### Edit and re-encrypt
For `project.md.sops` (binary), decrypt to plaintext, edit, then re-encrypt:
```bash
# decrypt to plaintext, edit it
sops -d --input-type binary --output-type binary project.md.sops > project.md

# re-encrypt to .sops using your age recipient
sops --encrypt --age "$(age-keygen -y ~/.config/sops/age/keys.txt)" \
  --input-type binary --output-type binary project.md > project.md.sops
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
- Enable admin by setting credentials:
  ```bash
  ADMIN_USERNAME=admin ADMIN_PASSWORD=changeme ADMIN_SESSION_SECRET=dev-secret \
  go run ./cmd/server
  ```
- Access:
  - Admin: http://localhost:8080/admin/ (login required)
  - Prometheus: http://localhost:9090
  - Grafana: http://localhost:3000 (anonymous access enabled via docker-compose for local dev)
  - Jaeger: http://localhost:16686

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
- App runs behind Docker Compose, with Postgres, Redis/Asynq, Qdrant, and Tika on internal network.

## Configuration
Environment variables (see `.env.sample`):
- Core: `APP_ENV`, `PORT`, `DB_URL`, `REDIS_URL`
- AI: `AI_PROVIDER`, `OPENROUTER_API_KEY`, `OPENAI_API_KEY`, etc.
- Vector DB: `QDRANT_URL`, `QDRANT_API_KEY`
- Extractor: `TIKA_URL`
- Observability: `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`
- Limits & CORS: `MAX_UPLOAD_MB`, `RATE_LIMIT_PER_MIN`, `CORS_ALLOW_ORIGINS`
