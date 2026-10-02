# SSO Rate Limiting and Brute Force Protection

This document describes the security measures implemented to protect the SSO login flow from abuse and brute force attacks.

## Overview

The AI CV Evaluator uses a multi-layered defense approach:

1. **Authelia Regulation (Brute Force Protection)** - Application-level account lockout
2. **Nginx Rate Limiting** - Network-level request throttling

---

## Authelia Regulation (Brute Force Protection)

Configured via Authelia configuration:

- Production: `deploy/authelia/configuration.prod.yml` (decrypted at runtime)
- Encrypted source of truth: `deploy/authelia/configuration.prod.sops.yml`

| Setting | Value | Description |
|---------|-------|-------------|
| `regulation.max_retries` | `100` | Max failures within the time window |
| `regulation.find_time` | `120` | Window in seconds for counting failures |
| `regulation.ban_time` | `120` | Ban time in seconds after threshold |

### Behavior

1. After too many failed login attempts within the window, access is temporarily denied
2. Lockout duration follows `regulation.ban_time`
3. Counter window follows `regulation.find_time`

---

## Nginx Rate Limiting

Configured in `deploy/nginx/nginx.conf` and `deploy/nginx/prod-conf.d/*.conf.template`:

### Rate Limit Zones

```nginx
# 10 requests/second per IP for general oauth2 endpoints
limit_req_zone $binary_remote_addr zone=oauth2_login_zone:10m rate=10r/s;

# 5 requests/second per IP for oauth2 callback (stricter)
limit_req_zone $binary_remote_addr zone=oauth2_callback_zone:10m rate=5r/s;
```

### Applied Limits

| Endpoint | Rate | Burst | Behavior |
|----------|------|-------|----------|
| `/oauth2/` | 10 req/s | 20 | Delayed excess requests |
| `/oauth2/callback` | 5 req/s | 5 | Immediate rejection (nodelay) |

### HTTP Response

- Excess requests receive **HTTP 429 Too Many Requests**
- Rate limit violations are logged at `warn` level

---

## Production Deployment

These protections are applied to:

- `ai-cv-evaluator.web.id` - Main API server
- `dashboard.ai-cv-evaluator.web.id` - Admin dashboard

### Configuration Files

- `deploy/authelia/configuration.prod.sops.yml` - Encrypted Authelia configuration
- `deploy/authelia/configuration.prod.yml` - Decrypted Authelia configuration used at runtime (not committed)
- `deploy/nginx/nginx.conf` - Rate limit zone definitions
- `deploy/nginx/prod-conf.d/app-https.conf.template` - API rate limits
- `deploy/nginx/prod-conf.d/dashboard.conf.template` - Dashboard rate limits

---

## Monitoring

Rate limit hits can be monitored via:

1. **Nginx logs** - Look for `limiting requests` warnings
2. **Authelia logs** - Look for regulation/ban events

---

## Testing

To verify rate limiting is working:

```bash
# Rapid requests should get throttled
for i in {1..50}; do
  curl -s -o /dev/null -w "%{http_code}\n" https://ai-cv-evaluator.web.id/oauth2/start
done
# Expect 429 responses after burst threshold
```

---

## Security Considerations

1. **No rate limiting in dev** - Only production configs have rate limits to avoid test interference
2. **IP-based limiting** - Uses `$binary_remote_addr` for efficiency
3. **Layered defense** - Both network (nginx) and application (Authelia) protection
