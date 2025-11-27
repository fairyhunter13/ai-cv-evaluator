# Developer Quick Reference

## Local development

- Prereqs: Docker, Docker Compose, Go 1.24+, Node.js 18+
- Copy or decrypt env:
  - `.env` for dev (OpenRouter/OpenAI keys, admin creds)
  - `.env.production` for prod

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
