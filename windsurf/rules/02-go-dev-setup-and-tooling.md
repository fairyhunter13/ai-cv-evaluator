---
trigger: always_on
---

Provide a productive Go developer experience with strict linting, formatting, and reproducible builds.

# Go Version and Modules
- Require Go 1.22+ via `go.mod`.
- Use modules; keep dependencies minimal and pinned.

# Tooling
- Linters via `golangci-lint` (enable: errcheck, gosec, govet, staticcheck, revive, ineffassign, typecheck, misspell, gocyclo configured sanely).
- Formatters: `gofmt` + `goimports`.
- Vulnerabilities: `govulncheck` in CI.
- Mocks: `gomock` + `mockgen` or `mockery`. Add `go:generate` directives near interfaces.
- Test runner: `gotestsum` for better CI output.

# Makefile Targets (guidance)
- `make deps` → install dev tools locally.
- `make fmt` → format and import fix.
- `make lint` → run golangci-lint with config.
- `make vet` → run `go vet ./...`.
- `make vuln` → run `govulncheck ./...`.
- `make test` → unit tests.
- `make test-int` → e2e tests (may spin containers).
- `make test-e2e` → end-to-end API tests.
- `make cover` → coverage report (HTML artifact).
- `make run` → run server using `.env`.
- `make docker-build`, `make docker-run` → container workflow.

# Environment & Config
- Parse envs into `internal/config` using `caarlos0/env` (or similar) with defaults and validation.
- Important envs:
  - `APP_ENV` (dev|staging|prod)
  - `PORT` (default 8080)
  - `DB_URL` (Postgres/SQLite DSN)
  - `REDIS_URL` (for queue)
  - `OPENROUTER_API_KEY` (required for live chat when using OpenRouter)
  - `OPENROUTER_BASE_URL` (default `https://openrouter.ai/api/v1`)
  - `CHAT_MODEL` (default `openrouter/auto` via OpenRouter)
  - `OPENAI_API_KEY` (required for embeddings)
  - `OPENAI_BASE_URL` (default `https://api.openai.com/v1`)
  - `EMBEDDINGS_MODEL` (default `text-embedding-3-small`)
  - `QDRANT_URL`, `QDRANT_API_KEY`
  - `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`
- Provide `.env.example` and README instructions.

## Secrets & Local .env Handling
- Never commit or modify the developer's local `.env` file from automation. Treat it as user-owned, ephemeral config.
- Credentials for OpenRouter/OpenAI are provided in the local `.env` (see the first lines for keys). Do not alter them. In CI, use GitHub Secrets; in code, rely on `config.Config` env parsing.
- Use SOPS for encrypted artifacts only (`.env.sops.yaml`, `project.md.sops`). When encrypting/decrypting locally, use `SOPS_AGE_KEY_FILE=~/.config/sops/age/keys.txt`.

# Repository Hygiene
- `.editorconfig` for consistent whitespace.
- `.gitignore` includes Go builds, vendor, .env, coverage, etc.
- Keep public symbols documented with GoDoc comments.

# golangci-lint Configuration (guidance)
- Provide `.golangci.yml` at repo root with:
  - `run: timeout: 5m`, enable linters listed above.
  - `issues:` exclude rules for generated files and vendor.
  - Reasonable `gocyclo` thresholds; avoid noisy false positives.
- In CI: `golangci-lint run ./...` with module cache warmed.

# Git Hooks / Pre-commit
- Use a `pre-commit` hook (or Husky-like alternative) to run:
  - `gofmt -l -w` and `goimports -w` on staged files.
  - `golangci-lint run` on changed packages.
  - Optional: `go test -short ./...` to catch quick regressions.
- Commit hooks live under `.githooks/` and are enabled via `core.hooksPath`.

# Test Placement Enforcement (Local)
- Unit tests MUST be co-located next to the code under test in the same package directory (e.g., `foo.go` ↔ `foo_test.go`).
- The top-level `test/` tree is reserved for E2E suites (`test/e2e/`) and shared fixtures only. Do not place unit tests under `test/`.
- **E2E Test Build Tags**: All E2E tests MUST have `//go:build e2e` and `// +build e2e` build tags
- **E2E Test Isolation**: E2E tests are excluded from regular test runs and require `-tags=e2e` flag
- Optional pre-commit guard to block misplaced tests:
  ```bash
  #!/usr/bin/env bash
  set -euo pipefail
  # Disallow *_test.go directly under top-level test/ except test/e2e/**
  if git ls-files -- '*.go' | grep -E '^test/.*_test\.go$' | grep -vE '^test/e2e/'; then
    echo "Error: unit tests must be colocated next to code. Move tests out of top-level test/ (allowed only under test/e2e/)." >&2
    exit 1
  fi
  # Verify E2E tests have build tags
  if git ls-files -- 'test/e2e/*.go' | xargs grep -L '//go:build e2e'; then
    echo "Error: E2E test files must have //go:build e2e build tag" >&2
    exit 1
  fi
  ```

# Makefile Guidance
- Export common flags: `GOFLAGS=-trimpath`, `CGO_ENABLED=0` (unless needed for PDF/DOCX), `GOTOOLCHAIN=auto`.
- Use pattern targets to test only changed packages when possible.
- Provide `make generate` to run `go generate ./...` for mocks and codegen.
- Provide `make tools` to install pinned versions of CLI tools into `./bin`.

# VS Code / Editor Settings
- Recommend enabling `editor.formatOnSave` and `go.formatTool` set to `gofmt`.
- Enable `gopls` features for staticcheck and vulncheck (if supported).
- Use workspace settings to set tab width and trim trailing whitespace.

# Code Generation
- Co-locate `//go:generate` directives next to interfaces for mocks.
- Regenerate mocks after interface changes and commit artifacts.

# Build & Cache Tips
- Warm module cache in CI: `go mod download` before lint/test.
- Use `actions/setup-go` with caching enabled; set `GOMODCACHE` for Docker builds.
- Prefer reproducible builds; vendor only if needed.

# Definition of Done (Dev Setup)
- Fresh clone can run `make deps fmt lint test` successfully.
- Config validation errors for missing critical envs.
