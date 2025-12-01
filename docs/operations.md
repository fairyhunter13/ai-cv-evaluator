# Operations Runbook

This document covers deployment, maintenance, and operational procedures for the AI CV Evaluator production environment.

## Architecture Overview

```
                    ┌──────────────┐
                    │   Nginx      │ (TLS termination, SSO gate)
                    └──────┬───────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│   Frontend    │  │   Backend     │  │   Grafana     │
│   (Vue.js)    │  │   (Go API)    │  │   (Metrics)   │
└───────────────┘  └───────┬───────┘  └───────────────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│  PostgreSQL   │  │   Redpanda    │  │    Qdrant     │
│   (Data)      │  │   (Queue)     │  │   (Vectors)   │
└───────────────┘  └───────┬───────┘  └───────────────┘
                           │
                           ▼
                   ┌───────────────┐
                   │    Worker     │
                   │  (Evaluator)  │
                   └───────────────┘
```

## Deployment

### Prerequisites

- Docker and Docker Compose installed
- Access to GHCR (GitHub Container Registry)
- SSL certificates provisioned via Let's Encrypt
- `.env.production` decrypted from SOPS

### Initial Deployment

```bash
# 1. Clone repository
git clone https://github.com/fairyhunter13/ai-cv-evaluator.git
cd ai-cv-evaluator

# 2. Decrypt secrets
make decrypt-env-production

# 3. Configure environment variables (edit if needed)
vim .env.production

# 4. Pull images
docker compose -f docker-compose.prod.yml pull

# 5. Start services
docker compose -f docker-compose.prod.yml up -d

# 6. Verify health
curl -k https://localhost/healthz
curl -k https://localhost/readyz
```

### Updating to New Version

```bash
# 1. Pull latest images (or specific tag)
docker compose -f docker-compose.prod.yml pull

# 2. Rolling update
docker compose -f docker-compose.prod.yml up -d --no-deps backend worker

# 3. Verify health
curl https://ai-cv-evaluator.web.id/healthz

# 4. If issues, rollback
docker compose -f docker-compose.prod.yml up -d --no-deps backend worker
# (specify previous image tag in docker-compose.prod.yml)
```

### Rollback Procedure

1. Identify previous working tag:
   ```bash
   docker images | grep ai-cv-evaluator
   ```

2. Update `docker-compose.prod.yml` with previous tag:
   ```yaml
   backend:
     image: ghcr.io/fairyhunter13/ai-cv-evaluator-server:v1.0.0  # Previous version
   ```

3. Deploy rollback:
   ```bash
   docker compose -f docker-compose.prod.yml up -d
   ```

## SSL Certificate Management

### Initial Setup (Let's Encrypt)

```bash
# 1. Ensure DNS points to server
dig ai-cv-evaluator.web.id

# 2. Run certbot for initial certificate
docker run -it --rm \
  -v /etc/letsencrypt:/etc/letsencrypt \
  -v /var/www/certbot:/var/www/certbot \
  certbot/certbot certonly \
  --webroot -w /var/www/certbot \
  -d ai-cv-evaluator.web.id \
  -d dashboard.ai-cv-evaluator.web.id

# 3. Reload nginx
docker compose -f docker-compose.prod.yml exec nginx nginx -s reload
```

### Certificate Renewal

Certificates auto-renew if certbot runs regularly. Set up cron:

```bash
# Add to crontab (runs at 3am daily)
0 3 * * * docker run --rm -v /etc/letsencrypt:/etc/letsencrypt -v /var/www/certbot:/var/www/certbot certbot/certbot renew --quiet && docker compose -f /path/to/docker-compose.prod.yml exec nginx nginx -s reload
```

### Manual Renewal

```bash
docker compose -f docker-compose.prod.yml run --rm certbot renew
docker compose -f docker-compose.prod.yml exec nginx nginx -s reload
```

## Secret Management

### Decrypting Secrets

```bash
# Requires SOPS_AGE_KEY_FILE to point to age key
export SOPS_AGE_KEY_FILE=~/.config/sops/age/keys.txt

# Decrypt production env
make decrypt-env-production
```

### Encrypting Updated Secrets

```bash
# After editing .env.production
make encrypt-env-production
```

### Rotating Secrets

1. Generate new values (API keys, passwords, session secrets)
2. Update `.env.production`
3. Re-encrypt:
   ```bash
   make encrypt-env-production
   ```
4. Redeploy affected services:
   ```bash
   docker compose -f docker-compose.prod.yml up -d backend worker keycloak oauth2-proxy-app oauth2-proxy-dashboard
   ```

## Scaling

### Horizontal Scaling (Workers)

```yaml
# In docker-compose.prod.yml
worker:
  deploy:
    replicas: 3  # Increase from 1
```

```bash
docker compose -f docker-compose.prod.yml up -d --scale worker=3
```

### Vertical Scaling

Adjust resource limits in compose file:

```yaml
worker:
  deploy:
    resources:
      limits:
        memory: 4G
        cpus: '4.0'
```

### Concurrency Tuning

```bash
# In .env.production
CONSUMER_MAX_CONCURRENCY=8  # Increase concurrent job processing
```

## Health Checks

### Endpoints

| Endpoint | Purpose | Expected Response |
|----------|---------|-------------------|
| `/healthz` | Liveness probe | `200 OK` |
| `/readyz` | Readiness probe | `200 OK` (includes dependency checks) |
| `/metrics` | Prometheus metrics | Prometheus format |

### Verifying Health

```bash
# All services healthy
docker compose -f docker-compose.prod.yml ps

# Specific service logs
docker compose -f docker-compose.prod.yml logs -f backend

# Application health
curl https://ai-cv-evaluator.web.id/healthz
curl https://ai-cv-evaluator.web.id/readyz
```

## Backup Procedures

### Database Backup

```bash
# Create timestamped backup
docker compose -f docker-compose.prod.yml exec db \
  pg_dump -U postgres app | gzip > backup_$(date +%Y%m%d_%H%M%S).sql.gz

# Store off-site (example: S3)
aws s3 cp backup_*.sql.gz s3://your-backup-bucket/postgres/
```

### Automated Backup Script

Create `/opt/scripts/backup-db.sh`:

```bash
#!/bin/bash
set -e

BACKUP_DIR=/opt/backups
DATE=$(date +%Y%m%d_%H%M%S)
COMPOSE_FILE=/home/hafiz/go/src/github.com/fairyhunter13/ai-cv-evaluator/docker-compose.prod.yml

mkdir -p $BACKUP_DIR

docker compose -f $COMPOSE_FILE exec -T db \
  pg_dump -U postgres app | gzip > $BACKUP_DIR/db_$DATE.sql.gz

# Keep last 30 days
find $BACKUP_DIR -name "db_*.sql.gz" -mtime +30 -delete
```

Add to cron:
```bash
0 2 * * * /opt/scripts/backup-db.sh >> /var/log/backup.log 2>&1
```

### Restore from Backup

```bash
# Stop application (optional but recommended)
docker compose -f docker-compose.prod.yml stop backend worker

# Restore
gunzip -c backup_20240101_020000.sql.gz | \
  docker compose -f docker-compose.prod.yml exec -T db psql -U postgres app

# Restart application
docker compose -f docker-compose.prod.yml start backend worker
```

## Troubleshooting

### Service Won't Start

```bash
# Check logs
docker compose -f docker-compose.prod.yml logs <service>

# Check resource usage
docker stats

# Verify dependencies
docker compose -f docker-compose.prod.yml ps
```

### Database Connection Issues

```bash
# Test connection
docker compose -f docker-compose.prod.yml exec db psql -U postgres -c "SELECT 1"

# Check connection count
docker compose -f docker-compose.prod.yml exec db psql -U postgres -c \
  "SELECT count(*) FROM pg_stat_activity"
```

### Queue Backlog

```bash
# Check Redpanda topic lag
docker compose -f docker-compose.prod.yml exec redpanda \
  rpk topic consume evaluation-jobs --print-timestamps -n 1

# Check consumer group lag
docker compose -f docker-compose.prod.yml exec redpanda \
  rpk group describe evaluation-consumer-group
```

### Memory Issues

```bash
# Check container memory
docker stats --no-stream

# Restart memory-heavy service
docker compose -f docker-compose.prod.yml restart worker
```

## Maintenance Windows

### Planned Maintenance

1. Announce maintenance window (status page, email)
2. Enable maintenance mode (if available):
   ```bash
   # Update nginx to return 503
   docker compose -f docker-compose.prod.yml exec nginx \
     sh -c "echo 'return 503;' > /etc/nginx/conf.d/maintenance.conf && nginx -s reload"
   ```
3. Perform maintenance
4. Disable maintenance mode:
   ```bash
   docker compose -f docker-compose.prod.yml exec nginx \
     sh -c "rm /etc/nginx/conf.d/maintenance.conf && nginx -s reload"
   ```
5. Verify services and announce completion

### Database Migrations

Migrations run automatically on startup via the `migrate` service. For manual migration:

```bash
# Run migrations
docker compose -f docker-compose.prod.yml run --rm migrate

# Verify
docker compose -f docker-compose.prod.yml exec db psql -U postgres app -c \
  "SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 5"
```

## Monitoring Alerts Response

See [Observability Runbook](./observability.md) for alert handling procedures.

## Emergency Contacts

| Role | Contact |
|------|---------|
| Platform Team | (define contact method) |
| Database Admin | (define contact method) |
| Security Team | (define contact method) |

## Revision History

| Date | Author | Changes |
|------|--------|---------|
| 2024-12-01 | AI Assistant | Initial version |
