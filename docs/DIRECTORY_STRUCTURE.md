# Directory Structure

High-level layout of the repository:

- `cmd/` – main entrypoints (if present) for server/worker binaries.
- `internal/` – application code following Clean Architecture:
  - `adapter/` – HTTP server, repositories (Postgres), queue (Redpanda), vector (Qdrant), observability.
  - `usecase/` – domain services orchestrating adapters.
  - `config/` – configuration parsing and defaults.
- `admin-frontend/` – Vue 3 + Vite admin UI:
  - `src/` – views, components, Pinia stores (including SSO auth store).
  - `tests/` – Playwright E2E tests for SSO gate semantics.
- `deploy/` – deployment-related configuration:
  - `nginx/` – dev and prod nginx vhost templates, including SSO/portal configuration.
  - `keycloak/` – Keycloak realm exports for dev/prod.
  - `portal/` – static portal HTML served behind SSO.
  - `grafana/`, `prometheus/`, `redpanda/`, etc. – observability configuration.
  - `docker/` – Dockerfiles for server, worker, migrate, and multi-target images.
- `secrets/` – SOPS-encrypted artifacts (env files, project brief, CV/RFC submissions, Keycloak realm, etc.).
- `test/e2e/` – Go-based E2E tests that exercise the full upload → evaluate → result pipeline.
- `.github/workflows/` – GitHub Actions for CI, security scanning, deploy, SSH checks, Cloudflare DNS sync.
- `docker-compose.yml` – local dev stack (including dev-nginx + SSO + dashboards).
- `docker-compose.prod.yml` – production stack.

See also:
- `docs/architecture/ARCHITECTURE.md` – system architecture overview.
- `docs/development/FRONTEND_DEVELOPMENT.md` – frontend development guide.
- `docs/security/SSO_RATE_LIMITING.md` – SSO rate limiting and brute force protection.
