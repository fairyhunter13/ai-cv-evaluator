# Comprehensive Progress Tracker

## 1. Project Overview
This document tracks the status of development, testing, and validation for the `ai-cv-evaluator` project. It serves as a living document to identify gaps, track issues, and guide future work.

## 2. CI/CD & Testing Status

| Component | Test Suite | Status | Last Run | Notes |
|-----------|------------|--------|----------|-------|
| **Authelia Login** | E2E (Playwright) | 🔴 Failing | v1.0.200 | Persistent CI failures despite local fixes. |
| **Backend API** | Go Test | 🟢 Passing | v1.0.200 | Core logic stable. |
| **Infrastructure** | Docker/Helm | 🟢 Passing | v1.0.200 | Containers build successfully. |
| **Deployment** | GitHub Actions | 🔴 Failing | v1.0.200 | Blocked by E2E validation. |

### 🔍 Recent Validation Failures
#### [ISSUE-2025-12-11-01] CI Playwright E2E Failure (Run 20140763191)
- **Status**: 🔴 Open / Investigation In Progress
- **Symptom**: `8` tests failed. Common error: `expect(locator).toBeVisible()` timeout (10000ms) for `input#username`.
- **Affected Tests**:
  - `logout clears session`
  - `dashboards reachable via portal after SSO login`
  - `backend API and health reachable via portal`
  - `logout flow redirects to login page`
- **Root Cause Analysis**:
  - **Trace**: Tests timed out waiting for the Authelia login form to appear after logout or initial redirection.
  - **Hypothesis A**: `AUTHELIA_URL` variable might be malformed in CI, causing API calls or redirects to fail.
  - **Hypothesis B**: "Secure" cookie logic might be strictly enforcing HTTPS in CI (if running behind a proxy?), preventing `http` callbacks.
  - **Hypothesis C**: UI slowness exceeds 10s default timeout.
- **Action Items**:
  1. [ ] Check CI environment variables for `AUTHELIA_URL`.
  2. [x] Increase UI timeout to 30s (Implemented in v1.0.201).
  3. [ ] Verify "Logout" URL construction.

## 3. Task Inventory

### ✅ Implemented & Verified
- [x] **Authelia Configuration**: Fixed `session.cookies` for v4.37.5 compatibility.
- [x] **Localhost Bypass**: Implemented `secure: false` cookie injection for local testing.
- [x] **Health Check**: Added `ensureAutheliaUp` to E2E specs (60s timeout).
- [x] **Env Awareness**: Refactored E2E to use `AUTHELIA_URL` variable.

### ⚠️ Implemented but Unstable
- [/] **Robust E2E Suite**:
  - *Current State*: New tests (Logout/Invalid Creds) introduced in v1.0.200.
  - *Regression*: 50% failure rate in CI.
  - *Goal*: Stabilize login form detection.

### 📝 Backlog / Gaps
- [ ] **Prod Verification**: Validate E2E tests against the production URL (`ai-cv-evaluator.web.id`).
- [ ] **Grafana Dashboard Tests**: "No Data" checks failed in previous runs; need mitigation.
- [ ] **Full Regression Suite**: Ensure all services (Jaeger, Prometheus, etc.) are reachable.

## 4. How to Test & Validate

### Local Development
```bash
# Start stack
docker compose up -d

# Run specific E2E test
cd admin-frontend
npx playwright test tests/sso-gate.spec.ts --project=chromium
```

### CI Simulation
To debug CI failures locally:
1. Prune all containers/volumes to ensure fresh state.
2. Run `make run-e2e-ci` (if available) or exactly mimic the `.github/workflows/ci.yml` steps.

## 5. References
- [Authelia v4.37 Docs](https://www.authelia.com/docs/configuration/session.html)
- [Playwright Network Guide](https://playwright.dev/docs/network)
- [GitHub Actions Runner Specs](https://docs.github.com/en/actions/using-github-hosted-runners/about-github-hosted-runners)
