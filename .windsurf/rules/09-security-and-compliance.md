---
trigger: always_on
---

Bake in security across inputs, dependencies, secrets, and runtime.

# Inputs & Files
- Allowlist: .txt, .pdf, .docx only; reject archives/executables.
- Max file size 10MB (configurable).
- Sanitize extracted text; strip control or RTL markers.

# AuthN/AuthZ (Optional if public)
- Protect write endpoints (POST) with token auth or JWT when exposed.

# Secrets & Config
- Do not commit plaintext secrets.
- Use SOPS to commit an encrypted secrets file (e.g., `.env.sops.yaml`) to the repo; keep the decrypted `.env` in `.gitignore`.
- Recommended: age as the SOPS key provider. Keep the age private key outside the repo; never commit it.
- CI/CD decrypts the SOPS file using a key provided via CI secrets and writes a runtime `.env`.
- Local dev: developers with access can decrypt to `.env` for testing. Never commit the decrypted file.

# Dependencies & Build
- `golangci-lint` + `govulncheck` in CI; block on issues.
- Container image uses minimal base; scan with `trivy`.

# Network & DoS
- Rate limit `/evaluate` per-IP; token bucket.
- Set server and client timeouts; cap body sizes.

# Logging & PII
- Structured JSON logs; include request id.
- Do not log full documents or secrets; redact large or sensitive fields.

# Threat Model & Trust Boundaries
- Public API surface limited to necessary endpoints; admin/ops endpoints gated.
- Trust boundaries clearly defined: client → API → queue/DB/vector DB → AI providers.
- Assume external systems (LLM provider) are untrusted; validate and sanitize all responses.

# HTTP Security Headers
- Send strict headers on all responses:
  - `X-Content-Type-Options: nosniff`
  - `X-Frame-Options: DENY` (or CSP `frame-ancestors 'none'`)
  - `Content-Security-Policy: default-src 'none'` (API returns JSON; restrict aggressively)
  - `Referrer-Policy: no-referrer`
  - `Strict-Transport-Security: max-age=63072000; includeSubDomains; preload` (HTTPS only)
- Ensure `Content-Type: application/json; charset=utf-8` for JSON.

# TLS & Network
- Enforce HTTPS end-to-end; redirect HTTP → HTTPS at the edge.
- Behind reverse proxy (Caddy/Nginx) terminate TLS with modern ciphers.
- Restrict inbound traffic via firewall to app port only; internal network for DB/Redis/Qdrant.

# Secrets Management & Rotation
- Never commit secrets. Use env variables and secret stores in prod.
- Rotate keys regularly; support hot-reload where feasible.
- Separate environment secrets per environment (dev/staging/prod).
- For VPS deploys, prefer SSH key-based authentication; store private key at `~/.ssh/id_rsa` with `600` permissions.

# Dependencies & Updates
- Pin versions in `go.mod`; run `go get -u=patch` periodically.
- Use dependency update automation (e.g., Renovate) with PR gating.
- `govulncheck` in CI blocks known vulnerabilities.

# SAST & DAST
- Static analysis via `golangci-lint` (gosec enabled) in CI.
- Optional DAST smoke via `zap-baseline.py` (OWASP ZAP) against staging.

# Data Protection & Compliance
- Minimize PII collection; document what is stored in `SECURITY.md`.
- Encrypt in transit (TLS) and at rest (DB storage encryption if available; disk encryption on VPS).
- Data retention policies: purge old uploads/results based on TTL.
- Provide data export/delete endpoints or scripts (subject access) if required.

# Secure Coding Practices
- Validate and sanitize all inputs; never trust file extensions alone.
- Use allowlists over denylists; prefer constants and enums.
- Handle errors without leaking internals to clients.
- Avoid command execution; if necessary, validate args strictly and sanitize.

# Incident Response
- On-call runbook with contact methods and triage steps.
- Log aggregation to identify spikes, errors, and suspicious activity.
- Ability to rotate keys, disable features via flags, and roll back quickly.

# Definition of Done (Security)
- Security checks run in CI.
- Rate limits and timeouts enforced.
- Inputs validated and sanitized end-to-end.
