---
trigger: always_on
---

Deliver high-confidence coverage with clear separation between unit, integration, and E2E tests.

# Unit Tests
- Table-driven tests with `t.Run(tc.name, func(t *testing.T) { ... })`; follow Arrange-Act-Assert for clarity.
- Use `t.Parallel()` where safe; avoid shared mutable state and test order dependencies.
- Assertions: prefer `testify/require` (fail-fast) and `assert` when multiple checks are useful.
- Package conventions:
  - Black-box tests for adapters and usecases in `package foo_test`.
  - White-box tests for internal helpers in `package foo` only when necessary.
- Mocks for external boundaries (LLM client, embeddings, Qdrant, Redis queue, DB repo).
  - Generate with `gomock` or `mockery`; store under `internal/<module>/mocks`.
  - Add `//go:generate` near interfaces to keep mocks up to date.
- Determinism:
  - Seed `math/rand` explicitly in tests.
  - Abstract time via `Clock`-like interface to control `time.Now()` in tests.
- Race detector and coverage:
  - Run unit tests with `-race` in CI on linux/amd64.
  - Collect coverage with `-coverprofile` and upload as artifact.
- Testdata management:
  - Put fixtures under `test/testdata/` (e.g., tiny .pdf/.docx for extraction).
  - Keep assets small and license-safe.
- Fast path: `go test -short ./...` runs pure unit tests (integration/E2E skipped via build tags).
- Coverage target ≥ 80% for core domain/usecase packages; ≥ 60% overall minimum.

# Unit Test Placement & Naming (Strict)
- Co-locate unit tests next to the code they test in the same package directory. Example: `internal/usecase/service.go` → `internal/usecase/service_test.go`.
- Do NOT place unit tests under the top-level `test/` tree. The `test/` tree is reserved for E2E suites (e.g., `test/e2e/`) and cross-cutting fixtures only.
- Package usage:
  - Black-box: `package foo_test` in the same directory to exercise only the public API.
  - White-box: `package foo` only when testing unexported helpers is necessary.
- Naming conventions:
  - File names end with `_test.go` (e.g., `service_test.go`).
  - Test functions use `TestXxx`, table-driven subtests use `t.Run(tc.name, ...)`.
- Test data:
  - Prefer a package-local `testdata/` subdirectory for package-specific fixtures.
  - Use `test/testdata/` only for shared, cross-cutting fixtures referenced by multiple packages.

# Integration Tests
- Use `testcontainers-go` to launch Postgres (v16+), Redis (v7+), and Qdrant.
- Apply migrations up in setup; teardown afterwards via `t.Cleanup`.
- Validate:
  - Upload text extraction using sample fixtures.
  - Queue consumption end-to-end (status transitions).
  - RAG retrieval returns expected chunks.
  - LLM mocked responses aggregate correctly.
  - Aggregation computes weighted scores per `project.md` (CV and Project components).
- Container orchestration details:
  - Use wait strategies (e.g., `wait.ForListeningPort()`) and health endpoints.
  - Build DSNs from container host/ports dynamically; pass via env (`DB_URL`, `REDIS_URL`, `QDRANT_URL`).
- AI interactions:
  - Default to deterministic mock clients unless `OPENAI_API_KEY` is provided.
  - Store fixtures under `test/testdata/ai_fixtures/`.
- Build tags and isolation:
  - Mark integration tests with `//go:build integration` (and legacy `+build integration`).
  - Avoid calling external networks; rely on containers only.
- Resource hygiene and timeouts:
  - Use `t.Cleanup` for container termination and DB teardown.
  - Keep suite under a few minutes; set per-test deadlines.

# E2E/API Tests
- Run full stack via testcontainers network (spawn app + deps) or `docker compose up -d` then hit the live app.
- Use `httpexpect` or `resty` to test:
  - `/upload` happy path and invalid cases (size, type, corrupt files).
  - `/evaluate` enqueues and returns `{id, status:"queued"}`.
  - `/result/{id}` for queued, processing, completed shapes (match `project.md`).
- Golden files for stable JSON responses under `test/testdata/golden/`.
- Completed result golden responses must include exactly the example fields from `project.md` (`cv_match_rate`, `cv_feedback`, `project_score`, `project_feedback`, `overall_summary`) with correct types.
- Build tags: mark E2E tests with `//go:build e2e` (and legacy `+build e2e`).
- Readiness:
  - Wait for health/readiness endpoint before assertions.
  - Use HTTP client with sensible timeouts; avoid flakiness.

# Reliability
- Deterministic seeds; bounded deadlines on IO.
- Prefer polling with backoff over fixed sleeps.
- Benchmarks for critical paths (optional).

# CI Integration (Testing)
- Unit tests (fast, race, coverage):
  - Run: go test -race -short -coverprofile=coverage.unit.out ./...
- Integration tests (containers, coverage, tagged):
  - Run: go test -tags=integration -coverprofile=coverage.int.out ./...
- E2E tests (optional in PRs, mandatory on main/tags):
  - Run: go test -tags=e2e ./test/e2e/...
- Upload coverage files and test logs as CI artifacts.

# Definition of Done (Testing)
- `make test` (unit with `-race`), `make test-int` (integration with containers), and E2E all pass locally and in CI.
