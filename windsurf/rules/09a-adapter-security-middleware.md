---
trigger: always_on
---

Adapter Security Middleware Cookbook for the HTTP layer (chi or similar).

# Middleware Chain (example)
- Recoverer (no stack traces to clients)
- Request ID injection (`X-Request-Id`)
- Timeout (per-route sensible defaults)
- CORS (allowlist; tighten in prod)
- Rate limit (token bucket/sliding window) for write endpoints
- Tracing (OpenTelemetry) and metrics (Prometheus)
- Security headers (see below)
- Access log (structured JSON via slog)

# Response Content Controls
- Enforce content-type `application/json; charset=utf-8` for API responses.
- Never emit chain-of-thought or step-by-step reasoning in responses; only return the defined JSON schema fields. See `04-ai-llm-and-rag-pipeline.md` → Chain-of-Thought (CoT) Handling.
- Apply response size caps and timeouts to prevent accidental leakage of large model outputs.
- On validation failures that indicate CoT leakage, return a standard error (e.g., `SCHEMA_INVALID`) without echoing the raw model output.

# Security Headers
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY` or CSP `frame-ancestors 'none'`
- `Content-Security-Policy: default-src 'none'`
- `Referrer-Policy: no-referrer`
- `Strict-Transport-Security: max-age=63072000; includeSubDomains; preload` (HTTPS only)
- All JSON responses: `Content-Type: application/json; charset=utf-8`

# File Upload Safety
- Enforce content sniffing; allowlist extensions (.txt/.pdf/.docx). Reject archives/executables.
- Limit multipart sizes; stream large files; strip control characters from extracted text.

# Caching & ETags
- Allow `GET /result/{id}` to use `ETag`/`If-None-Match` for completed results.

# Definition of Done (Adapter Security)
- Middleware stack applied; headers verified in e2e/E2E tests.
- Rate limiting and timeouts enforced on mutating endpoints.
