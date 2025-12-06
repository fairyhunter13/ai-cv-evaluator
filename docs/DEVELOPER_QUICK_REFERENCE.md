# Developer Quick Reference

## Local development

- Prereqs: Docker, Docker Compose, Go 1.24+, Node.js 18+
- Copy or decrypt env:
  - `.env` for dev (OpenRouter/OpenAI keys, admin creds)
  - `.env.production` for prod
  - Recommended: use SOPS + Make targets backed by `secrets/env.sops.yaml` and `secrets/env.production.sops.yaml`:
    ```bash
    make decrypt-env            # secrets/env.sops.yaml -> .env
    make decrypt-env-production # secrets/env.production.sops.yaml -> .env.production
    ```

- Groq and OpenRouter chat model selection is automatic; chat models are not configured via environment variables.

### Start full dev stack (backend + worker + SSO + dashboards)

```bash
make dev-full
```

This brings up:

- API server (`app`) on internal port 8080
- Worker (`worker`) processing queue jobs
- Postgres, Redpanda, Qdrant, Tika, OTEL collector
- **Keycloak** (IdP) on host port `8089`
- **oauth2-proxy** in front of nginx
- **dev-nginx** on host `http://localhost:8088` acting as a single SSO gate
- Portal UI under `/` with links to:
  - Admin frontend (`/app/`)
  - Backend API (`/v1/`)
  - Grafana, Prometheus, Jaeger, Redpanda console

All dashboards and admin/API endpoints are protected by SSO via oauth2-proxy + Keycloak.

## Testing

### Go unit / integration tests

```bash
make test      # go test ./...
make ci-test   # used by CI, includes coverage gate
```

### E2E tests (Go, backend worker + live AI providers)

```bash
make test-e2e  # requires OPENROUTER_API_KEY and OPENAI_API_KEY (optional fallback: OPENROUTER_API_KEY_2)
```

- CI uses the same E2E suite via the `run-e2e-tests` target in the deploy workflow, with safe defaults for worker concurrency and timeouts. Only increase these values if your AI quotas can handle it.

- By default (when `RUN_FULL_SMOKE_E2E` is *not* set), the E2E suite runs a trimmed set of comprehensive, edge-case, and performance tests that are tuned for free-tier stability. Heavier/noisier cases (e.g. `Noisy_Data` edge case, additional large pairs) only run when you explicitly enable full smoke:

  ```bash
  RUN_FULL_SMOKE_E2E=1 make test-e2e   # same suite as CI, but explicitly enabling heavier smoke locally
  ```

### Playwright SSO gate tests (frontend)

From `admin-frontend/`:

```bash
npm install            # once
npx playwright install # once
npm run test:e2e       # runs tests in tests/sso-gate.spec.ts
```

These tests assume `make dev-full` is running and verify that:

- Unauthenticated access to `/app/`, `/grafana/`, `/prometheus/`, `/jaeger/`, `/redpanda/`, `/admin/` is redirected into the SSO flow.
- After logging in once via SSO, those dashboards are reachable without further logins.
- `/logout` revokes SSO, and protected URLs again send you to SSO.

## Deployment

- Production stack is defined in `docker-compose.prod.yml`.
- Public entrypoint is the Nginx container, which frontends the API, frontend, SSO, and dashboards.
- CI/CD is orchestrated via GitHub Actions workflows in `.github/workflows/`.

### Deployment Pipeline

The deploy workflow enforces strict quality gates:

1. **Pre-deploy checks** - Requires semantic version tags (v1.2.3)
2. **Security gate** - CI and Docker Publish workflows must succeed
3. **E2E verify** - Smoke tests must pass
4. **Deploy** - Blue/green deployment with automatic rollback
5. **Production validation** - Health checks, Playwright E2E, alerting validation
6. **Cloudflare DNS sync** - Automatic DNS record updates

### Creating a Release

```bash
# Commit your changes
git add .
git commit -m "feat: your feature description"
git push origin main

# Create and push a semantic version tag to trigger deploy
git tag v1.0.125
git push origin v1.0.125
```

## Coverage Gate

The CI workflow enforces an 80% minimum code coverage gate via `make ci-test`. This is checked locally by:

```bash
make ci-test  # Fails if coverage < 80%
```

Note: Codecov may show a different percentage due to its own calculation method. The authoritative gate is the `make ci-test` target which uses `go tool cover`.

## Observability

### Local Development URLs

| Service | URL |
|---------|-----|
| Frontend | http://localhost:3001 |
| Backend API | http://localhost:8080 |
| SSO Portal | http://localhost:8088 |
| Grafana | http://localhost:3000 |
| Prometheus | http://localhost:9090 |
| Jaeger | http://localhost:16686 |
| Redpanda Console | http://localhost:8090 |

### Production URLs

| Service | URL |
|---------|-----|
| Production Site | https://ai-cv-evaluator.web.id |
| Admin Dashboard | https://ai-cv-evaluator.web.id/app/dashboard |
| Grafana | https://ai-cv-evaluator.web.id/grafana/ |
| Prometheus | https://ai-cv-evaluator.web.id/prometheus/ |
| Jaeger | https://ai-cv-evaluator.web.id/jaeger/ |
| Mailpit | https://ai-cv-evaluator.web.id/mailpit/ |
