# SSO Rate Limiting and Brute Force Protection

This document describes the security measures implemented to protect the SSO login flow from abuse and brute force attacks.

## Overview

The AI CV Evaluator uses a multi-layered defense approach:

1. **Keycloak Brute Force Protection** - Application-level account lockout
2. **Nginx Rate Limiting** - Network-level request throttling

---

## Keycloak Brute Force Protection

Configured via a SOPS-managed realm file:

- Encrypted source: `secrets/deploy/keycloak/realm-aicv.json.sops`
- Decrypted runtime file (gitignored): `deploy/keycloak/realm-aicv.json`

| Setting | Value | Description |
|---------|-------|-------------|
| `bruteForceProtected` | `true` | Enables brute force detection |
| `failureFactor` | `5` | Number of failed attempts before lockout |
| `permanentLockout` | `false` | Temporary lockout (not permanent) |
| `waitIncrementSeconds` | `60` | Initial lockout duration (1 minute) |
| `maxFailureWaitSeconds` | `900` | Maximum lockout duration (15 minutes) |
| `minimumQuickLoginWaitSeconds` | `60` | Minimum wait for quick login attempts |
| `quickLoginCheckMilliSeconds` | `1000` | Threshold for detecting rapid attempts |
| `maxDeltaTimeSeconds` | `43200` | Reset counter after 12 hours of no failures |

### Behavior

1. After **5 failed login attempts**, the account is temporarily locked
2. Initial lockout is **1 minute**, increasing progressively
3. Maximum lockout is **15 minutes**
4. Counter resets after **12 hours** of no failed attempts
5. Users see a lockout message in Keycloak login page

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

- `secrets/deploy/keycloak/realm-aicv.json.sops` - Encrypted Keycloak realm (brute force settings + admin user)
- `deploy/keycloak/realm-aicv.json` - Decrypted Keycloak realm used at runtime (not committed)
- `deploy/nginx/nginx.conf` - Rate limit zone definitions
- `deploy/nginx/prod-conf.d/app-https.conf.template` - API rate limits
- `deploy/nginx/prod-conf.d/dashboard.conf.template` - Dashboard rate limits

---

## Monitoring

Rate limit hits can be monitored via:

1. **Nginx logs** - Look for `limiting requests` warnings
2. **Keycloak admin console** - View locked accounts under Users > Brute Force

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
3. **Layered defense** - Both network (nginx) and application (Keycloak) protection
4. **Progressive lockout** - Keycloak increases lockout duration to discourage persistent attacks
