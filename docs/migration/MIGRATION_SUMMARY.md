# Migration Summary

This document briefly captures the main migration steps that were performed in this repository.

## Highlights

- Migrated to a split architecture with dedicated `server`, `worker`, and `frontend` containers.
- Consolidated worker logic into a single worker handling all Redpanda partitions, tuned for free-tier-safe concurrency by default (`CONSUMER_MAX_CONCURRENCY=1`, can be increased in higher-capacity environments).
- Introduced Authelia + oauth2-proxy SSO in front of all dashboards and admin/frontend routes.
- Added a portal page behind SSO that acts as a single entry point to:
  - Admin frontend (`/app/`)
  - Backend API (`/v1/`)
  - Grafana, Prometheus, Jaeger, Redpanda console
- Hardened configuration defaults in `internal/config/config.go` so that most env vars are optional overrides.
- Minimized `.env` / `.env.production` and corresponding SOPS files to only required secrets (AI keys + admin credentials).
- Added GitHub Actions workflows for:
  - CI (linting, tests, coverage gate)
  - Deploy (build, scan, E2E tests, SSH deploy)
  - SSH connectivity tests
  - Cloudflare DNS synchronization
- Added Playwright-based SSO tests to validate portal + dashboard access rules.

For more details, see the architecture and development docs referenced from `docs/README.md`.
