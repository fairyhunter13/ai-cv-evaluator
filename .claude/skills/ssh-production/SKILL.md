---
name: ssh-production
description: Connect to ai-cv-evaluator production server via SSH and run diagnostics
---

# SSH Production Server

## Connection Details

- **Host**: 43.157.225.155
- **Username**: ubuntu
- **Private Key**: ~/.ssh/id_rsa
- **Deploy Directory**: ~/ai-cv-evaluator/

## Connect

```bash
ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=no -o IdentitiesOnly=yes -i ~/.ssh/id_rsa ubuntu@43.157.225.155
```

## Common Diagnostics

### Check all Docker containers

```bash
ssh -o IdentitiesOnly=yes -i ~/.ssh/id_rsa ubuntu@43.157.225.155 "cd ~/ai-cv-evaluator && docker compose -f docker-compose.prod.yml ps"
```

### Check disk space (common cause of failures)

```bash
ssh -o IdentitiesOnly=yes -i ~/.ssh/id_rsa ubuntu@43.157.225.155 "df -h && docker system df"
```

### Check container logs

```bash
ssh -o IdentitiesOnly=yes -i ~/.ssh/id_rsa ubuntu@43.157.225.155 "cd ~/ai-cv-evaluator && docker compose -f docker-compose.prod.yml logs --tail=50 <service>"
```

Services: `nginx`, `backend_blue`, `backend_green`, `frontend`, `worker`, `authelia`, `oauth2-proxy-app`, `postgres`, `qdrant`, `tika`, `prometheus`, `grafana`, `certbot`, `health-monitor`

### Check container resource usage

```bash
ssh -o IdentitiesOnly=yes -i ~/.ssh/id_rsa ubuntu@43.157.225.155 "docker stats --no-stream"
```

### Restart a service

```bash
ssh -o IdentitiesOnly=yes -i ~/.ssh/id_rsa ubuntu@43.157.225.155 "cd ~/ai-cv-evaluator && docker compose -f docker-compose.prod.yml restart <service>"
```

### Clean up Docker (recover disk space)

```bash
ssh -o IdentitiesOnly=yes -i ~/.ssh/id_rsa ubuntu@43.157.225.155 "docker system prune -af --volumes 2>/dev/null; docker image prune -af"
```

### Check nginx config and reload

```bash
ssh -o IdentitiesOnly=yes -i ~/.ssh/id_rsa ubuntu@43.157.225.155 "cd ~/ai-cv-evaluator && docker compose -f docker-compose.prod.yml exec nginx nginx -t && docker compose -f docker-compose.prod.yml exec nginx nginx -s reload"
```

## Gotchas

- **fail2ban**: Server has fail2ban with maxretry=3, bantime=3600s. Always use `-o IdentitiesOnly=yes -i ~/.ssh/id_rsa` to avoid trying multiple keys which triggers the ban.
- **Blue/Green deploy**: Active backend color is in `~/ai-cv-evaluator/.active_color`. Check which is active before restarting.
- **SOPS encryption**: Authelia configs and env files are SOPS-encrypted in the repo. On the server they're already decrypted.
