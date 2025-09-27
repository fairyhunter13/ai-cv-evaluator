---
trigger: always_on
---

Provide a concrete path to deploy online via a Docker-capable VPS.

# VPS Strategy
- Requirements on VPS: Docker, docker compose, ports open for app and Postgres/Redis/Qdrant if remote.
- Directory on VPS: `$REMOTE_COMPOSE_PATH` (e.g., `/opt/ai-cv-evaluator`).
- Compose file includes app, db, redis, qdrant; app uses image from GHCR.

# GitHub Actions Deploy (non-blocking, no quotes)
- Secrets needed: `SSH_HOST`, `SSH_USER`, `SSH_PRIVATE_KEY`, `REMOTE_COMPOSE_PATH`, `IMAGE_REF`, optional `GHCR_USERNAME`, `GHCR_TOKEN`.
- Example remote steps (each non-interactive and non-blocking):

Run: ssh -o StrictHostKeyChecking=no $SSH_USER@$SSH_HOST mkdir -p $REMOTE_COMPOSE_PATH
Run: ssh -o StrictHostKeyChecking=no $SSH_USER@$SSH_HOST docker login ghcr.io -u $GHCR_USERNAME -p $GHCR_TOKEN
Run: ssh -o StrictHostKeyChecking=no $SSH_USER@$SSH_HOST docker pull $IMAGE_REF
Run: ssh -o StrictHostKeyChecking=no $SSH_USER@$SSH_HOST docker compose -f $REMOTE_COMPOSE_PATH/docker-compose.yml up -d
Run: ssh -o StrictHostKeyChecking=no $SSH_USER@$SSH_HOST docker run --rm --network host $IMAGE_REF make migrate

- Ensure `up -d` to prevent stuck jobs; avoid quotes in run steps.

# Post-Deploy Verification
- Health endpoint responds 200.
- Smoke tests for `/upload`, `/evaluate`, `/result/{id}` using mock mode when keys absent.

# Release Management
- Tags `vX.Y.Z` trigger image publish and deploy.
- Rollback by re-deploying previous tag.

# Server Provisioning & Prerequisites
- OS: Ubuntu LTS (recommended) with a non-root user in `docker` group.
- Install Docker and docker compose plugin from official repos; enable on boot.
- Create application directory on VPS: `$REMOTE_COMPOSE_PATH` (e.g., `/opt/ai-cv-evaluator`).
- Networking:
  - Open the app port only; keep DB/Redis/Qdrant on an internal network.
  - Optional reverse proxy (Caddy/Nginx) terminates TLS and forwards to app.

# Remote Compose & Files
- Place `docker-compose.yml` under `$REMOTE_COMPOSE_PATH` with services:
  - `app`: image from GHCR, healthcheck, restart unless-stopped.
  - `db`, `redis`, `qdrant`: with named volumes and healthchecks.
  - Optional `otel-collector`, `jaeger`, `prometheus` via a compose profile.
- Place a production `.env` on the VPS (permissions 600). Do not commit this file.
- Ensure `docker compose` reads the `.env` file; no secrets committed to repo.

# Migrations & Ordering
- Run DB migrations before starting the new app image to avoid schema drift.
- Suggested order in deploy job:
  - `docker pull $IMAGE_REF`
  - `docker run --rm --network host $IMAGE_REF make migrate`
  - `docker compose -f $REMOTE_COMPOSE_PATH/docker-compose.yml up -d`
- Treat migrations as idempotent and backward compatible when possible.

# Rollback Plan
- Keep previous tag available in GHCR.
- To roll back:
  - `docker pull ghcr.io/<owner>/<repo>:<previous>`
  - `IMAGE_REF=ghcr.io/<owner>/<repo>:<previous>` then re-run compose up -d.
- If a migration caused issues, provide a down migration or hotfix forward migration.

# Logs & Monitoring on VPS
- Use `docker compose logs --tail 300` for quick inspection; pipe to files for support bundles.
- Consider a lightweight log forwarder (e.g., vector/fluent-bit) to ship JSON logs.
- Expose Prometheus metrics endpoint and scrape via external Prometheus if available.

# Zero-Downtime Considerations
- Single-instance compose typically incurs a brief restart during deploy.
- To minimize downtime:
  - Put a reverse proxy in front and briefly drain connections.
  - Run two app replicas on different ports behind the proxy; update one at a time.
  - Alternatively use a platform with rolling deploys (e.g., Fly.io) for full zero-downtime.

# SSH Key-based Authentication
- Use SSH keys for all deploy connections.
- Place private key at `~/.ssh/id_rsa` (permissions 600) and public key at `~/.ssh/id_rsa.pub` on the VPS `authorized_keys`.
- In CI, load the key into the runner (see CI rules) and avoid interactive prompts.

# Definition of Done (Deploy)
- One-click deploy via GitHub Action updates VPS.
- Secrets managed; environment configured correctly.
