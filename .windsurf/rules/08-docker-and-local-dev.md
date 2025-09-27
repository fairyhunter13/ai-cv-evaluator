---
trigger: always_on
---

Containerize the app for local dev, testing, and production.

# Dockerfile (Multi-Stage)
- Builder: `golang:1.22-bookworm`, cache go modules and build.
- Build flags: `-ldflags=-s -w`; disable CGO unless needed; static binary if possible.
- Runtime: `gcr.io/distroless/base-debian12` or `alpine:3.20` when shell utilities needed.
- Run as non-root; expose `$PORT`.

# docker-compose (Local Dev)
- Services:
  - `app`: depends_on db, redis, qdrant; env from `.env`; bind port `${PORT:-8080}`.
  - `db`: Postgres with volume and healthcheck.
  - `redis`: for queue.
  - `qdrant`: vector DB.
  - Optional: `otel-collector`, `jaeger`, `prometheus`.
- Commands include `up -d` to avoid blocking.

# Image Configuration
- Reads env vars documented in `.env.example`.
- Graceful shutdown on SIGTERM; readiness/liveness endpoints.

# Dockerfile Details
- Use multi-stage:
  - builder: cache `GOMODCACHE` and `GOCACHE` to speed builds.
  - final: copy only the built binary and minimal runtime files.
- Run as non-root user with a fixed UID/GID; set `UMASK` appropriately.
- Use `tini` as entrypoint in Alpine to handle PID 1 signals if shell is required.
- Provide `.dockerignore` to exclude `.git`, `test/`, local artifacts, and `**/*.md` (keep `api/openapi.yaml`).
- Example healthcheck (Alpine runtime): `HEALTHCHECK CMD wget -qO- http://localhost:$PORT/healthz || exit 1`.

# Compose Profiles & Services
- Use Compose profiles for optional services (e.g., `observability`).
- Define named volumes for Postgres and Qdrant data to persist across restarts.
- Healthchecks:
  - `db`: use `pg_isready -U postgres`.
  - `redis`: `redis-cli ping` expecting `PONG`.
  - `qdrant`: GET `http://qdrant:6333/collections`.
- App depends_on with `condition: service_healthy` (Compose v2) to wait for deps.

# Qdrant in Compose (Vector DB)
- Recommended service definition (example):
  ```yaml
  qdrant:
    image: qdrant/qdrant:latest
    environment:
      - QDRANT__SERVICE__API_KEY=${QDRANT_API_KEY:-}
    volumes:
      - qdrant_data:/qdrant/storage
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:6333/collections"]
      interval: 10s
      timeout: 3s
      retries: 10
    # Keep Qdrant internal; do not expose in prod. Compose default network is enough.
    # ports:
    #   - "6333:6333" # optional for local debugging only
  ```
- App configuration for Qdrant:
  - `QDRANT_URL=http://qdrant:6333`
  - `QDRANT_API_KEY` set to match the container env (optional in dev, required when enabled)
- Do not expose Qdrant outside the internal network in production; access from app only.

# Env & Secrets Management
- Local: use `.env` loaded by compose; do not commit `.env`.
- Production: use server-side `.env` with restricted permissions or Docker secrets.
- Never pass secrets via CLI args or commit them in the repo.

# Using SOPS for Local Dev
- You may commit an encrypted secrets file (e.g., `.env.sops.yaml`) with SOPS.
- Developers who have access can decrypt locally:
  ```bash
  sops -d .env.sops.yaml > .env
  chmod 600 .env
  ```
- Do not commit the decrypted `.env`. Keep `.env` in `.gitignore`.

# Networking & Ports
- Bind service ports from env: `${PORT:-8080}`; avoid hardcoding.
- Use an internal network for service-to-service communication; expose only the app port externally.

# Performance Tips
- Enable build cache mounts in Dockerfile for Go modules to speed CI builds.
- Reduce image size: strip symbols, use distroless when shell not required.
- Prefer `COPY --chown=<uid>:<gid>` to avoid root-owned files in the container.

# Definition of Done (Docker)
- Image builds reproducibly; minimal size.
- `docker compose up -d` starts full stack locally.
