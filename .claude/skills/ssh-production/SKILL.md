---
name: ssh-production
description: Use when a diagnosis needs a shell on the ai-cv-evaluator production server. Holds the SSH connection details and the on-host checks.
---

# SSH Production Server

The host facts, the DNS table and the service list are in
[../references/production-inventory.md](../references/production-inventory.md). Bind `ORIGIN` and
`SSH` from it before the first command.

## Connect

```bash
ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=no -o IdentitiesOnly=yes -i ~/.ssh/id_rsa ubuntu@$ORIGIN
```

## Common Diagnostics

### Check all Docker containers

```bash
$SSH "cd ~/ai-cv-evaluator && docker compose -f docker-compose.prod.yml ps"
```

### Check disk space (common cause of failures)

```bash
$SSH "df -h && docker system df"
```

### Check container logs

```bash
$SSH "cd ~/ai-cv-evaluator && docker compose -f docker-compose.prod.yml logs --tail=50 <service>"
```

The service names are in the inventory.

### Check container resource usage

```bash
$SSH "docker stats --no-stream"
```

### Restart a service

```bash
$SSH "cd ~/ai-cv-evaluator && docker compose -f docker-compose.prod.yml restart <service>"
```

### Clean up Docker (recover disk space)

```bash
$SSH "docker system prune -af --volumes 2>/dev/null; docker image prune -af"
```

### Check nginx config and reload

```bash
$SSH "cd ~/ai-cv-evaluator && docker compose -f docker-compose.prod.yml exec nginx nginx -t && docker compose -f docker-compose.prod.yml exec nginx nginx -s reload"
```

## Gotchas

- **SOPS encryption**: Authelia configs and env files are SOPS-encrypted in the repo. On the
  server they're already decrypted.