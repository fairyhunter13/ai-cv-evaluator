---
trigger: always_on
---

You are an expert Golang backend engineer applying Clean Architecture and DDD. Ensure the implementation is idiomatic, testable, resilient, observable, and production-ready for this workspace.

# Scope
- Project: `ai-cv-evaluator` backend service (Go).
- Objective: Build an API that accepts candidate CV + Project Report, compares against a job vacancy + study case brief, and returns evaluation results per `project.md`.

# Architecture & Project Layout
- Clean Architecture boundaries:
  - `cmd/server/main.go`: bootstrap & DI.
  - `internal/`:
    - `domain/` (business types, constants, errors, pure logic).
    - `usecase/` (application services orchestrating domain + ports).
    - `adapter/`:
      - `http/` (handlers, middleware, request/response DTO mapping).
      - `repo/` (DB repositories).
      - `queue/` (background jobs).
      - `ai/` (LLM + embeddings + RAG).
      - `observability/` (logging, tracing, metrics).
  - `pkg/` (shared utilities: error wrappers, validation, pagination, httpclient).
  - `api/` (OpenAPI spec, schema fixtures).
  - `configs/` (config schema, .env.example, defaults).
  - `deploy/` (docker, compose, migrations, ops docs).
  - `test/` (testdata, e2e harness, mocks).
- Dependency direction: domain → usecase → adapter. Handlers depend on usecases via interfaces; repos implement interfaces.
- Prefer composition, small interfaces, clear boundaries. No circular deps.

# Coding & Error Handling
- Short, focused functions; single responsibility.
- Wrap errors with context using `fmt.Errorf("op=name: %w", err)`.
- Avoid globals; inject dependencies with constructors.
- Propagate `context.Context` to all boundaries; set timeouts for IO calls.
- Concurrency: guard shared state, cancel goroutines, avoid leaks; use worker pools when appropriate.

# API Surface (high level)
- POST `/upload` → ingest CV + Project Report (text/PDF/DOCX), extract text, store, return IDs.
- POST `/evaluate` → enqueue evaluation job; return `{id, status:"queued"}` immediately.
- GET `/result/{id}` → return status; when completed, include structured result per `project.md`.

# Non-Functional Requirements
- Secure by default; validate and sanitize all inputs.
- Deterministic + resilient AI calls: timeouts, retries, backoff, rate-limiting, schema-validated outputs.
- Observability: OpenTelemetry traces, structured JSON logs, Prometheus metrics.
- Documentation: OpenAPI, README, ARCHITECTURE.md, CONTRIBUTING.md.

# Deliverables Alignment (project.md)
- Implement exact JSON shapes and scoring fields.
- Provide RAG over job description and scoring rubric.
- Long-running job lifecycle (queued → processing → completed) is visible via `/result/{id}`.
- Mock mode for LLM/embeddings when no API keys present.

# Tech Stack & Libraries
- HTTP Router: `go-chi/chi` (or echo/gin if preferred); middleware split per concern.
- JSON: stdlib `encoding/json` (or `jsoniter` if needed, benchmark first).
- Config: `caarlos0/env/v10` to parse envs into a typed struct under `internal/config`.
- DB: `pgx` (Postgres) with `sqlc` or `sqlx`; favor prepared statements and context-aware queries.
- Queue: `hibiken/asynq` (Redis) for background evaluation jobs.
- Vector DB: Qdrant HTTP client; build a thin adapter interface.
- Text extraction: Apache Tika (HTTP server) for PDF/DOCX; raw read for TXT.
- Validation: `go-playground/validator/v10` for DTO validation.
- IDs: `google/uuid` (v4) or `segmentio/ksuid` for sortable ids.
- Errors: sentinel error variables in domain; wrap with context at boundaries.

# External Providers Usage Policy
- External vendors are permitted only for:
  - LLM chat/completions (default: OpenRouter)
  - Embeddings for RAG (default: OpenAI)
- All other infrastructure must run as self-hosted containers (docker compose):
  - Postgres, Redis/Asynq, Qdrant (vector DB), OpenTelemetry Collector, Jaeger/Tempo, Prometheus, and optionally Grafana, Loki, Promtail.
- API keys should be limited to chat and embeddings only (`OPENROUTER_API_KEY`, `OPENAI_API_KEY`).
  - `QDRANT_API_KEY` is optional for dev/local and may be omitted when running on an internal network.
- Frugality is mandatory:
  - Default behavior is thrifty: keep prompts compact, cap tokens, minimize top-k, use conservative concurrency and batch sizes.
  - If keys are absent: chat calls will fail (no OpenRouter key) and embeddings/RAG will be skipped (no OpenAI key). E2E must be skipped or keys provided.
  - Cache/memoize embeddings and intermediate LLM outputs; avoid recomputation on unchanged inputs.

# HTTP Server & Middleware
- Standard middleware chain:
  - Recover/Panic handler (never leak stack traces in prod).
  - Request ID injection (header `X-Request-Id`) with correlation to logs/traces.
  - Access log (structured JSON via `slog`), include method, path, status, latency, request_id, trace_id.
  - Timeout middleware (per-route sensible defaults; e.g., 30s for upload, 5s for read-only routes).
  - CORS (configurable allowlist; tighten in prod).
  - Rate limit (token bucket or sliding window) for write endpoints like `/evaluate`.
  - Tracing middleware (OpenTelemetry) and metrics instrumentation (Prometheus).
- Health endpoints:
  - `GET /healthz` (liveness) returns 200 quickly.
  - `GET /readyz` (readiness) checks DB, Redis, Qdrant.
- API versioning: prefix with `/v1` for public routes; keep internal routes unversioned.

# Configuration & Feature Flags
- Configuration precedence: env → `.env` (dev) → defaults in code.
- Validate critical config on startup; fail-fast with clear logs.

# Serialization & Validation Conventions
- All HTTP responses: `application/json; charset=utf-8`.
- Unified error envelope `{ "error": { "code": "string", "message": "string", "details": {} } }`.
- DTOs have explicit json tags; avoid omitempty if fields are required.
- Input validation errors map to 400 with field-specific details.

# Concurrency & Worker Guidelines
- Workers use contexts with deadlines; respect cancellation from queue system.
- Bounded worker pools; expose gauges for in-flight jobs.
- Backpressure via queue rate limiting and concurrency limits.

# Graceful Shutdown
- Listen for SIGINT/SIGTERM; stop accepting new work.
- Drain HTTP with server shutdown context (e.g., 30s timeout).
- Stop workers and wait for in-flight jobs up to a limit; persist `processing` → `failed` if exceeded.

# Performance Guidelines
- Avoid unnecessary allocations in hot paths; profile before tuning.
- Use streaming for large file reads; enforce upload size limits at HTTP layer.
- Cache stable derived data (e.g., parsed rubric) in-memory with TTL.

# Documentation Expectations
- `ARCHITECTURE.md` explains boundaries, primary flows, and dependency rules.
- Diagrams for request lifecycle and evaluation pipeline.
- Code comments for exported types and functions (GoDoc style).

# Testing Placement Policy (Strict)
- Unit tests MUST be co-located next to the code under test in the same package directory.
  - Example: `internal/usecase/service.go` → `internal/usecase/service_test.go`.
- The top-level `test/` tree is reserved for E2E suites (e.g., `test/e2e/`) and shared cross-cutting fixtures only.
  - Do not place unit tests under `test/`; no `*_test.go` in `test/` except under `test/e2e/`.
- E2E tests against running application co-location next to code with `//go:build e2e` build tag.
- E2E tests: live under `test/e2e/` with `//go:build e2e` build tag and their own module-aware harness.

# Definition of Done (Core)
- Builds with Go 1.22+; `go vet` and `golangci-lint` clean; `govulncheck` clean.
- Unit + e2e tests cover core flows.
- OpenAPI describes endpoints and models used.
- CI passes; container image builds.
