---
trigger: always_on
---

Define both observability dashboards and an admin UI for testing/consuming APIs.

# Observability Dashboards
- Metrics (Prometheus) with dashboards for:
  - HTTP performance and errors by route
  - Job queue depth, processing, DLQ
  - AI latency/error rates by provider
- Tracing (Jaeger/Tempo) with spans across HTTP → queue → worker → AI/Qdrant.
- Document dashboard links and basic runbooks in `docs/`.

# Trace–Log Correlation Dashboard (Grafana)
- Data sources: Tempo (or Jaeger via data source) and Loki.
- Panels (example):
  - Panel 1: Trace view → input variable `trace_id` and a search panel to select by recent traces.
  - Panel 2: Logs for the same trace → Loki query filtering on label `trace_id` and/or field `request_id`.
- Promtail: parse JSON logs, extract labels `trace_id`, `request_id`, `service`.
- Annotations: add annotations for deploys/releases to aid incident timelines.

# Admin UI (Behind Login)
- Purpose: allow authenticated users to test and consume the API from a browser UI.
- Tech: Tailwind CSS for styling, minimal HTML + vanilla JS for interactivity (no heavy framework required).
- Routes (examples):
  - `/admin/login` → username/password form
  - `/admin/` → dashboard home (links to Upload, Evaluate, Results)
  - `/admin/upload` → form with two file inputs (`cv`, `project`); POST to `/upload`
  - `/admin/evaluate` → textarea/inputs for `cv_id`, `project_id`, `job_description`, `study_case_brief`; POST to `/evaluate`
  - `/admin/result` → input for `job_id`; poll `/result/{id}` and render JSON
- UX & Behavior:
  - Use Tailwind utility classes for responsive layout.
  - Show loading states and error toasts; display structured JSON results.
  - Keep requests within the same origin; include `X-Request-Id` and other headers as needed.
- Security:
  - Require login via session cookie (HTTP-only, Secure) before accessing `/admin/*` routes.
  - CSRF tokens on form pages.
  - Rate-limit login; audit login attempts.
- Deployment:
  - Admin UI can be served by the same Go app (embedded templates) or as static files behind the app.
  - Disable or restrict admin UI in public environments via feature flag.

# Definition of Done (Admin & Dashboards)
- Authenticated admin can upload, start evaluation, and view results via UI.
- Observability dashboards show baseline metrics and traces.
- Admin UI styled with Tailwind and functional across supported browsers.
