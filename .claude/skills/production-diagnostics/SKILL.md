---
name: production-diagnostics
description: Use when ai-cv-evaluator production is down, slow or returning errors, and you must find out why. Walks the health check, Cloudflare, Docker and SSL layers in order.
---

# Production Diagnostics

The host facts, the DNS table and the service list are in
[../references/production-inventory.md](../references/production-inventory.md). Bind `ORIGIN` and
`SSH` from it before the first command.

## Quick Triage (no SSH needed)

### 1. Check backend health

```bash
# Direct to origin (fast, bypasses Cloudflare)
curl -sS -k https://$ORIGIN/healthz -H "Host: ai-cv-evaluator.web.id"
curl -sS -k https://$ORIGIN/readyz -H "Host: ai-cv-evaluator.web.id"
```

### 2. Check Authelia (SSO)

```bash
curl -sS -k https://$ORIGIN/api/health -H "Host: auth.ai-cv-evaluator.web.id"
```

### 3. Measure Cloudflare latency

```bash
# Through Cloudflare
curl -sS -w "TTFB: %{time_starttransfer}s Total: %{time_total}s\n" -o /dev/null https://ai-cv-evaluator.web.id/healthz

# Direct (should be <200ms)
curl -sS -k -w "TTFB: %{time_starttransfer}s Total: %{time_total}s\n" -o /dev/null https://$ORIGIN/healthz -H "Host: ai-cv-evaluator.web.id"
```

### 4. Test full redirect chain

```bash
curl -sS -L -w "Redirects: %{num_redirects} Total: %{time_total}s HTTP: %{http_code}\n" -o /dev/null https://ai-cv-evaluator.web.id/
```

### 5. Check static asset loading (blank screen diagnostic)

```bash
# Authelia JS bundle (~569KB) - if this fails, login page is blank
curl -sS -k -o /dev/null -w "HTTP: %{http_code} Size: %{size_download}\n" https://$ORIGIN/static/js/index.CHT8JlKb.js -H "Host: auth.ai-cv-evaluator.web.id"
```

### 6. Check SSL certificate

```bash
echo | openssl s_client -connect $ORIGIN:443 -servername ai-cv-evaluator.web.id 2>/dev/null | openssl x509 -noout -dates
```

## SSH Diagnostics (when needed)

See skill: `ssh-production` for connection details.

```bash
# Disk space (most common issue)
$SSH "df -h && docker system df"

# Container status
$SSH "cd ~/ai-cv-evaluator && docker compose -f docker-compose.prod.yml ps"

# Recent container restarts
$SSH "docker ps --format '{{.Names}} {{.Status}}' | sort"

# nginx error log
$SSH "cd ~/ai-cv-evaluator && docker compose -f docker-compose.prod.yml logs --tail=30 nginx"

# Check which backend is active (blue/green)
$SSH "cat ~/ai-cv-evaluator/.active_color 2>/dev/null || echo 'blue (default)'"
```

## Cloudflare Diagnostics

See skill: `cloudflare-management` for API commands.

Key checks:

- Security level should be `medium` (not `under_attack`)
- Browser check should be `off` if causing slowness
- Super Bot Fight Mode: dashboard-only on free plan

## Common Issues

| Symptom                      | Likely Cause                           | Fix                                       |
| ---------------------------- | -------------------------------------- | ----------------------------------------- |
| Blank login page             | Authelia JS bundle failing to load     | Check nginx proxy buffers, disk space     |
| Very slow page load (10-30s) | Cloudflare challenges on every request | Disable Under Attack Mode, Bot Fight Mode |
| 502 Bad Gateway              | Backend container down                 | `docker compose restart backend_blue`     |
| SSL error                    | Certificate expired                    | Check certbot logs, renew manually        |
| Can't SSH                    | fail2ban IP ban (1h)                   | Wait 1 hour, or use VPS provider console  |
| OAuth2 redirect loop         | Authelia or oauth2-proxy misconfigured | Check logs for both containers            |
