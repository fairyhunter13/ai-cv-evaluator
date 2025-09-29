- [x] Integration tests removed/disabled; E2E only.
  - Evidence: `internal/usecase/integration_test.go` and `internal/integration/containers_test.go` marked `//go:build ignore`.
### Docs Maintenance (New)
- [x] README: Go version updated to 1.24+.
- [x] Testing Rules: E2E terminology clarified; UI E2E policy added.
- [x] Dev rules: Never modify local `.env`; use SOPS with `SOPS_AGE_KEY_FILE` and GitHub Secrets in CI.
  - Evidence: `windsurf/rules/02-go-dev-setup-and-tooling.md` updated.

# AI CV Evaluator — Comprehensive TODOs

Status legend:
- [x] Completed (implemented in the repo; verified by code references)
- [ ] Pending (to implement)
- [~] In progress / optional improvements

This plan aligns with `project.md` (API and evaluation pipeline requirements) and all guidelines under `windsurf/rules/`.

Last updated: 2025-09-28 09:40 +07

---

## P0 — Core Architecture & Contracts (Rules: 01, 03)

- [x] Clean Architecture boundaries (`internal/domain`, `internal/usecase`, `internal/adapter`, `cmd/server`) established.
  - Evidence: `cmd/server/main.go`, `internal/domain/entities.go`, `internal/usecase/*`, `internal/adapter/*`.
- [x] Contract-first OpenAPI describing endpoints and schemas.
  - Evidence: `api/openapi.yaml`; served by `GET /openapi.yaml` in `internal/adapter/httpserver/handlers.go`.
- [x] Implement endpoints per project.md
  - [x] POST `/v1/upload` (multipart: `cv`, `project`) → returns `{cv_id, project_id}`.
    - Evidence: `UploadHandler()` in `internal/adapter/httpserver/handlers.go`.
  - [x] POST `/v1/evaluate` (JSON) → returns `{id, status:"queued"}`.
    - Evidence: `EvaluateHandler()` in `internal/adapter/httpserver/handlers.go`.
  - [x] GET `/v1/result/{id}` shows status; on complete returns `result` object per spec.
    - Evidence: `ResultHandler()` + `ResultService.Fetch()` assembling envelope.
- [x] Unified error envelope `{ "error": { "code", "message", "details" } }` and status mapping.
  - Evidence: `internal/adapter/httpserver/responses.go`.
- [x] ETag/If-None-Match supported for GET results.
  - Evidence: `ResultService.Fetch()` computes ETag.
- [x] GET `/v1/result/{id}` failed-shape (recommended by rules) returns `{id, status:"failed", error:{code,message}}` when job failed.
  - Evidence: `ResultService.Fetch()` includes error object; `api/openapi.yaml` has Failed schema; `handlers_result_test.go` tests it.

## P1 — Storage, Queueing, and Data Model (Rules: 05, 13)

- [x] Database schema and migrations (uploads, jobs, results).
  - Evidence: `deploy/migrations/20250927122000_init.sql`.
- [x] Repositories for uploads, jobs, results using `pgxpool`.
  - Evidence: `internal/adapter/repo/postgres/*.go`.
- [x] Queueing with Redis/Asynq, worker executing evaluation pipeline.
  - Evidence: `internal/adapter/queue/asynq/{queue,worker}.go`.
- [x] Idempotency key support on `/evaluate` (store on job; return existing on duplicates).
  - Evidence: `EvaluateService.Enqueue()` + `jobs.idempotency_key` column.
  - Evidence: `internal/adapter/repo/postgres/cleanup.go` with `RunPeriodic()`; config flags `DATA_RETENTION_DAYS`, `CLEANUP_INTERVAL`.

## P2 — AI Pipeline, Prompting, and RAG (Rules: 04, 04a, 04b)

- [x] AI providers: Real-only (OpenRouter for chat, OpenAI for embeddings). No stub/mock for E2E.
  - Evidence: `cmd/server/main.go` always wires real OpenRouter client; `internal/adapter/ai/stub/client.go` disabled via `//go:build ignore`.
## Coverage Uplift Plan (to reach ≥80% overall)
- [x] httpserver: add tests covering more branches (Accept mismatch, size/type rejections, JSON validation paths, ETag 304).
- [x] repo/postgres: add repository tests for error cases and happy paths.
- [x] queue/asynq: add enqueue tests with mocks and error branches.
- [x] config/observability: add minimal tests to lift totals.
- [x] golden tests for prompt I/O; schema enforcement on `parseAndNormalize`.
  - Evidence: `internal/adapter/ai/real/client.go` has robust JSON parsing and retry logic.
- [x] Two-pass prompting (normalize/consistency pass) as per rules.
  - Evidence: `FEATURE_TWO_PASS_LLM` flag; `buildNormalizationSystemPrompt()` in `eval_json.go`; worker implements second pass.
- [x] RAG stores job description and scoring rubric in Qdrant; retrieval applied in worker.
  - Evidence: `internal/app/qdrant.go` seeding; worker `topTextsByWeight()` and `buildUserWithContext()` use weighted retrieval.
- [x] LLM Chaining implemented (extract → evaluate-from-extracts + RAG).
{{ ... }}
- [x] Idempotency key support for duplicate prevention
- [x] Health checks for all external dependencies
- [x] Comprehensive error handling with structured responses



## Cross-Reference Index (Where to change)

- API Handlers: `internal/adapter/httpserver/handlers.go`, `responses.go`, `middleware.go`
- Usecases: `internal/usecase/{upload,evaluate,result}.go`
- Domain model & errors: `internal/domain/entities.go`
- Queue & Worker: `internal/adapter/queue/asynq/{queue,worker,eval_json}.go`
- AI Clients: `internal/adapter/ai/real/client.go`
- Vector DB (Qdrant): `internal/adapter/vector/qdrant/client.go`
- Text Extractor (Tika): `internal/adapter/textextractor/tika/tika.go`
- Observability: `internal/adapter/observability/{logger,metrics,tracing}.go`
- DB Repos: `internal/adapter/repo/postgres/{uploads_repo,jobs_repo,results_repo}.go`
- Migrations: `deploy/migrations/*.sql`
{{ ... }}
- CI/CD: `.github/workflows/*.yml`
- Config: `internal/config/config.go`, `configs/.env.example`, `.env.sample`

## Refactors & Cleanups (New)
- [x] Remove deprecated admin cookie helpers and duplicate file extraction helper.
  - Evidence: Cleaned from `internal/adapter/httpserver/handlers.go`; use `SessionManager` in `auth.go` and `AdminServer` only.
- [x] Fix `extractUploadedText` to call external extractor for `.pdf`/`.docx` and sanitize text for `.txt`.
  - Evidence: Updated implementation in `handlers.go`.
- [x] Remove unused `pkg/textx.ExtractFromPath`; rely on Tika for PDF/DOCX.
  - Evidence: `pkg/textx/textx.go` simplified to `SanitizeText` only.
- [x] Remove stub/mock AI for E2E; real providers only.
  - Evidence: stub client disabled via `//go:build ignore`.

---

## Acceptance Test Checklist (High-level)

- __Upload__: Accepts .txt/.pdf/.docx; rejects mislabeled binaries via content sniffing; <= MaxUploadMB; returns ids.
- __Evaluate__: Validates required fields and lengths; enqueues job; idempotency works with `Idempotency-Key`.
- __Result__: Queued/Processing/Completed/Failed shapes match OpenAPI; ETag works; 304 on If-None-Match.
- __RAG__: Retrieval returns relevant seeds for job and rubric corpora in E2E using live providers.
- __AI__: JSON-only outputs; 1–3 sentence feedback fields; 3–5 sentence summary; retries on schema issues.
- __Observability__: `/metrics` exposes HTTP, job, AI metrics; traces visible in Jaeger; logs structured with `request_id`.
- __Security__: Rate limiting applied; strict headers; no raw prompts or secrets in logs.
- __CI__: Lint, vet, vulncheck, tests, OpenAPI validation, image build, publish on tag, deploy on demand.
- __Docs__: README, ARCHITECTURE.md updates, STUDY_CASE.md present, PR/issue templates in place.
