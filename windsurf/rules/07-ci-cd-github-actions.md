---
trigger: always_on
---

Provide CI/CD with GitHub Actions covering linting, tests, security, containerization, and deploy to a VPS.

# CI Workflows
- `.github/workflows/ci.yml` (on PR + push):
  - Setup Go with caching.
  - Run format check, `golangci-lint`, `go vet`, `govulncheck`.
  - Unit tests with coverage artifact.
  - Integration tests using services (Postgres, Redis, Qdrant) or `testcontainers`.
  - Build multi-stage Docker image; upload as artifact for reuse.

- `.github/workflows/docker-publish.yml` (on tag):
  - Build and push image to GHCR: `ghcr.io/<owner>/<repo>:<tag>` and `:latest`.
  - Optionally generate SBOM and sign.

- `.github/workflows/deploy.yml` (manual or tag):
  - Target: VPS with Docker and docker-compose.
  - Required secrets: `SSH_HOST`, `SSH_USER`, `SSH_PRIVATE_KEY`, `REMOTE_COMPOSE_PATH`, `IMAGE_REF`.
  - Steps outline:
    - Setup SSH agent.
    - Connect to VPS.
    - Login to GHCR if private.
    - Pull image, run migrations, and `docker compose up -d`.

# VPS Deploy Notes (avoid stuck commands)
- Use non-interactive commands and `-d` for compose to avoid blocking.
- Avoid quotes in run steps. Example snippets for a job step:

Run: ssh -o StrictHostKeyChecking=no $SSH_USER@$SSH_HOST mkdir -p $REMOTE_COMPOSE_PATH
Run: ssh -o StrictHostKeyChecking=no $SSH_USER@$SSH_HOST docker login ghcr.io -u $GHCR_USERNAME -p $GHCR_TOKEN
Run: ssh -o StrictHostKeyChecking=no $SSH_USER@$SSH_HOST docker pull $IMAGE_REF
Run: ssh -o StrictHostKeyChecking=no $SSH_USER@$SSH_HOST docker compose -f $REMOTE_COMPOSE_PATH/docker-compose.yml up -d
Run: ssh -o StrictHostKeyChecking=no $SSH_USER@$SSH_HOST docker run --rm --network host $IMAGE_REF make migrate

- Ensure remote compose uses `restart: unless-stopped` and healthchecks.

# Tests in CI (explicit commands)
- Unit (fast, race, coverage):
  - Run: go test -race -short -coverprofile=coverage.unit.out ./...
- Integration (containers with tags):
  - Run: go test -tags=e2e -coverprofile=coverage.int.out ./...
- E2E (toggle on main/tags):
  - Run: go test -tags=e2e ./test/e2e/...
- Artifacts:
  - Upload coverage files and test logs for all steps.

# Test Placement Policy (CI check)
- Add a CI step to enforce unit tests are co-located next to code and not under the top-level `test/` tree (except `test/e2e/`).
- Example GitHub Actions step:
  ```yaml
  - name: Enforce unit test placement
    shell: bash
    run: |
      set -euo pipefail
      # Find any *_test.go under top-level test/ except test/e2e/**
      if git ls-files -- ':!:test/e2e/**' 'test/**/_test.go' | grep -E '.+'; then
        echo "Misplaced unit tests detected under top-level test/. Move unit tests next to code (e.g., pkg/foo/foo_test.go)." >&2
        exit 1
      fi
  ```

# Caching & Performance
- Use `actions/setup-go` with module and build cache.
- Warm cache: run go mod download before lint/test.
- Docker buildx cache-from and cache-to for faster image builds.

# Matrix & Concurrency
- Run matrix on Go versions (e.g., 1.22, 1.23) and platforms (linux/amd64 primary).
- Use `concurrency` group per-branch to auto-cancel superseded workflows.

# Security Scans
- `govulncheck` as part of CI.
- Container scan (e.g., `trivy`) on the built image in publish workflow.

# Naming Hygiene (CI)
- Enforce naming/documentation hygiene to avoid sensitive or external brand terms.
- Maintain a denylist file at `tools/naming_denylist.txt` (one term per line). Keep this file private to your org if needed.
- Example GitHub Actions step:
  ```yaml
  - name: Naming hygiene check
    shell: bash
    run: |
      set -euo pipefail
      if [ -f tools/naming_denylist.txt ]; then
        terms=$(grep -vE '^\s*(#|$)' tools/naming_denylist.txt || true)
        if [ -n "$terms" ]; then
          echo "$terms" | while IFS= read -r term; do
            if grep -RIn --exclude-dir=.git --binary-files=without-match -- "$term" .; then
              echo "Found disallowed term in repo content: $term" >&2
              exit 1
            fi
          done
        fi
      fi
  ```

# Secrets with SOPS (CI usage)
- You may commit an encrypted secrets file (e.g., `.env.sops.yaml`) to the repo using SOPS with age keys; do NOT commit a plaintext `.env`.
- In CI, decrypt it using a private age key provided via repository secrets, then write `.env` for steps that require it.
- Example steps (Ubuntu runner):
  ```yaml
  - name: Install sops
    run: |
      sudo apt-get update
      sudo apt-get install -y sops

  - name: Prepare age key for SOPS
    env:
      SOPS_AGE_KEY: ${{ secrets.SOPS_AGE_KEY }}
    run: |
      mkdir -p ~/.config/sops/age
      printf "%s" "$SOPS_AGE_KEY" > ~/.config/sops/age/keys.txt
      chmod 600 ~/.config/sops/age/keys.txt

  - name: Decrypt env
    run: |
      sops -d .env.sops.yaml > .env
      chmod 600 .env
  ```
- Do not print secret values in logs. Avoid `set -x`. Restrict `.env` file permissions.

# Deploy via SSH Agent (keys, no quotes)
- Load SSH key using an SSH agent action (recommended) or via run steps writing the key to `~/.ssh/id_rsa` with correct permissions.
- Ensure subsequent `ssh` and `scp` commands are non-interactive and have `-o StrictHostKeyChecking=no`.
- Example preparatory step (no quotes):

Run: mkdir -p ~/.ssh && chmod 700 ~/.ssh
Run: printf %s $SSH_PRIVATE_KEY > ~/.ssh/id_rsa
Run: chmod 600 ~/.ssh/id_rsa
Run: ssh -o StrictHostKeyChecking=no $SSH_USER@$SSH_HOST echo ok

# Quality Gates
- Fail CI on lint or vuln issues.
- Enforce coverage thresholds for core packages.
- Deploy is gated on successful CI.

# Definition of Done (CI/CD)
- CI runs automatically on PRs.
- Image published on tags to GHCR.
- Deploy workflow updates VPS successfully with non-blocking commands.

# OpenAPI Validation (CI)
- Validate `api/openapi.yaml` early in CI to catch contract drifts:
  - Run: go run github.com/getkin/kin-openapi/cmd/validate@latest api/openapi.yaml
  - Alternative (Node): npx @redocly/cli@latest lint api/openapi.yaml --max-problems=0
  - Alternative (Docker): docker run --rm -v $PWD:/work redocly/cli lint /work/api/openapi.yaml
- Gate merges on a clean spec; keep contract as source of truth for handlers and tests.
