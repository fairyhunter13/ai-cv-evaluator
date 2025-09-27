---
trigger: always_on
---

Authentication and authorization for public APIs and the admin portal.

# Public API (Optional Protection)
- Keep `/upload`, `/evaluate`, `/result/{id}` open for local/dev.
- For staging/prod, protect write endpoints with token auth or JWT as needed.
- Apply per-IP rate limits and sensible timeouts to deter abuse.

# Admin Portal Auth
- Admin area behind username/password login.
- Passwords stored with Argon2id (or bcrypt with strong cost); never in plaintext.
- Session management:
  - Signed, HTTP-only, Secure cookies.
  - CSRF protection for form posts.
- Account roles: `admin`, `viewer`. Restrict write operations to `admin`.
- Login throttling: lockout/backoff after repeated failures.

# API Keys (Optional)
- Support per-user API tokens for the admin portal to script flows.
- Store hashed tokens; allow rotation and revocation.

# Definition of Done (Auth)
- Admin routes require login and role checks.
- Security headers and CSRF enforced in authenticated flows.
- Secrets are not logged; credentials never stored in plaintext.
