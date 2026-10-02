# Production inventory

One home for the host facts the four production skills share. A number written in four files is
four chances to drift.

## Bind the two names first

Every command in those skills reads `$ORIGIN` or `$SSH`. Run this before the first one:

```bash
export ORIGIN=43.157.225.155
export SSH="ssh -o IdentitiesOnly=yes -i ~/.ssh/id_rsa ubuntu@$ORIGIN"
```

## Host

| Field | Value |
| --- | --- |
| Origin IP | `43.157.225.155` |
| SSH user | `ubuntu` |
| Private key | `~/.ssh/id_rsa` |
| Deploy directory | `~/ai-cv-evaluator/` |
| Domain | `ai-cv-evaluator.web.id` |

`IdentitiesOnly=yes` is not optional. fail2ban runs with `maxretry=3` and `bantime=3600s`, and a
client that offers several keys reaches that count on its own.

## DNS records, managed by Terraform

| Subdomain | Type | Target | Purpose |
| --- | --- | --- | --- |
| `@` (root) | A | `43.157.225.155` | Main application |
| `dashboard` | A | `43.157.225.155` | Admin dashboard |
| `auth` | A | `43.157.225.155` | Authelia SSO |

`keycloak.ai-cv-evaluator.web.id` also resolves. Every A record is proxied, so a direct call to the
origin needs `-k` and an explicit `Host` header.

Terraform owns these records. Change one through `terraform/cloudflare`, never by hand.

## Compose services

`nginx`, `backend_blue`, `backend_green`, `frontend`, `worker`, `authelia`, `oauth2-proxy-app`,
`postgres`, `qdrant`, `tika`, `prometheus`, `grafana`, `certbot`, `health-monitor`.

The active backend colour is in `~/ai-cv-evaluator/.active_color`. Read it before you restart one.
