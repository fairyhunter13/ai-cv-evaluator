---
name: cloudflare-management
description: Use when a DNS record, a proxy setting or a security setting must change on ai-cv-evaluator.web.id, or when you must read the current one. Talks to the Cloudflare API.
---

# Cloudflare Management

The host facts, the DNS table and the service list are in
[../references/production-inventory.md](../references/production-inventory.md). Bind `ORIGIN` and
`SSH` from it before the first command.

## Credentials

- **API Token**: stored in `.env.production` as `CLOUDFLARE_API_TOKEN`
- **Zone ID**: stored in `.env.production` as `CLOUDFLARE_ZONE_ID`
- **Account ID**: stored in `.env.production` as `CLOUDFLARE_ACCOUNT_ID`
- **Domain**: ai-cv-evaluator.web.id

## Load Credentials

```bash
source .env.production
# Token: $CLOUDFLARE_API_TOKEN
# Zone: $CLOUDFLARE_ZONE_ID
```

## Verify Token

```bash
curl -sS -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
  "https://api.cloudflare.com/client/v4/user/tokens/verify"
```

## DNS Records

### List all DNS records

```bash
curl -sS -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
  "https://api.cloudflare.com/client/v4/zones/$CLOUDFLARE_ZONE_ID/dns_records" \
  | python3 -c "import sys,json; [print(f\"{r['name']:40s} {r['type']:5s} {r['content']:20s} proxied={r.get('proxied','?')}\") for r in json.load(sys.stdin).get('result',[])]"
```

### Create/Update DNS record

```bash
curl -sS -X POST -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
  -H "Content-Type: application/json" \
  "https://api.cloudflare.com/client/v4/zones/$CLOUDFLARE_ZONE_ID/dns_records" \
  -d '{"type":"A","name":"subdomain","content":"'"$ORIGIN"'","proxied":true}'
```

## Security Settings

### Check security level

```bash
curl -sS -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
  "https://api.cloudflare.com/client/v4/zones/$CLOUDFLARE_ZONE_ID/settings/security_level"
```

### Change security level (off, essentially_off, low, medium, high, under_attack)

```bash
curl -sS -X PATCH -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
  -H "Content-Type: application/json" \
  "https://api.cloudflare.com/client/v4/zones/$CLOUDFLARE_ZONE_ID/settings/security_level" \
  -d '{"value":"medium"}'
```

### Check/toggle browser check

```bash
# Check
curl -sS -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
  "https://api.cloudflare.com/client/v4/zones/$CLOUDFLARE_ZONE_ID/settings/browser_check"

# Disable
curl -sS -X PATCH -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
  -H "Content-Type: application/json" \
  "https://api.cloudflare.com/client/v4/zones/$CLOUDFLARE_ZONE_ID/settings/browser_check" \
  -d '{"value":"off"}'
```

## Quick Health Check (no auth needed)

```bash
# Through Cloudflare
curl -sS -w "\nTTFB: %{time_starttransfer}s\n" https://ai-cv-evaluator.web.id/healthz

# Direct to origin (bypass Cloudflare)
curl -sS -k -w "\nTTFB: %{time_starttransfer}s\n" https://$ORIGIN/healthz -H "Host: ai-cv-evaluator.web.id"

# Cloudflare trace
curl -sS https://ai-cv-evaluator.web.id/cdn-cgi/trace
```

## Gotchas

- **Token permissions**: The token in `.env.production` needs Zone:DNS:Edit AND Zone:Settings:Edit
  for full management. If security settings return 9109 Unauthorized, the token needs to be
  updated in Cloudflare dashboard.
- **Super Bot Fight Mode**: Only configurable via Cloudflare dashboard (not API) on free plans. If
  JS challenges are injected, check Security > Bots in dashboard.
- **Proxied records**: every A record stays `proxied=true`. The origin IP and the subdomain list
  are in the inventory.