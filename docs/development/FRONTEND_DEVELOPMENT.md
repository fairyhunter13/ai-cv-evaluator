# Frontend Development

The admin frontend lives under `admin-frontend/` and is a Vue 3 + Vite SPA.

## Local dev server

From `admin-frontend/`:

```bash
npm install    # once
npm run dev    # Vite dev server on port 3001
```

When running the full dev stack via `make dev-full`, nginx (`dev-nginx`) proxies:

- `http://localhost:8088/app/` → frontend dev server (HMR enabled).
- Other paths (`/v1/`, `/grafana/`, `/prometheus/`, `/jaeger/`, `/redpanda/`) to backend and dashboards.

## Authentication and SSO Integration

- All admin/frontend routes are behind SSO via oauth2-proxy + Keycloak.
- The auth store (`admin-frontend/src/stores/auth.ts`) provides:
  - `login` / `loginWithSSO(redirectTo?)`: triggers `/oauth2/start?rd=...`.
  - `logout`: redirects to `/oauth2/sign_out?rd=/` and clears local state.
  - `checkAuth`: calls `/admin/api/status` to detect current auth status.
- By default, `loginWithSSO` redirects back to the **portal root** (`/`), ensuring a consistent post-login entry point.

## API Usage

- The frontend uses Axios with relative paths (`/admin/api/...`, `/v1/...`).
- Nginx terminates TLS (in prod) and forwards requests to the backend API container.

## E2E / SSO Tests with Playwright

Playwright config and tests live under `admin-frontend/`:

- `playwright.config.ts`: sets `testDir = ./tests` and `baseURL = http://localhost:8088`.
- `tests/sso-gate.spec.ts`: verifies SSO gate semantics.

Running the tests:

```bash
# In repo root
make dev-full

# In admin-frontend/
npm install
npx playwright install
npm run test:e2e
```

The SSO tests assert that:

- Unauthenticated users are redirected into SSO when hitting `/app/`, `/grafana/`, `/prometheus/`, `/jaeger/`, `/redpanda/`, `/admin/`.
- After successful SSO login (Keycloak), the user lands on the portal root and can open dashboards without re-authenticating.
- `/logout` revokes SSO and protected URLs again require login.
