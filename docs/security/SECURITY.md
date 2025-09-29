# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability, please follow these steps:

1. **DO NOT** create a public GitHub issue
2. Email security concerns to: security@example.com (update with actual email)
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

We will acknowledge receipt within 48 hours and provide updates on the fix progress.

## Security Measures

### Authentication & Authorization
- Admin UI protected by session-based authentication
- Session cookies with HttpOnly, Secure, and SameSite flags
- Login rate limiting to prevent brute force
- Configurable session secrets via environment variables

### Input Validation
- File type validation (extension and MIME type checking)
- Upload size limits enforced
- Request body size limits on all endpoints
- JSON schema validation for API requests
- SQL injection prevention via parameterized queries

### Secrets Management
- SOPS encryption for sensitive configuration
- Environment variables for runtime secrets
- No hardcoded credentials in codebase
- Separate secrets per environment

### Network Security
- CORS configuration with origin allowlisting
- Security headers (X-Content-Type-Options, X-Frame-Options, CSP)
- HTTPS enforced in production (via reverse proxy)
- Rate limiting on mutating endpoints

### Data Protection
- Sanitization of user inputs
- No raw SQL queries
- Prepared statements for database operations
- Context-aware output encoding

### Dependencies
- Regular vulnerability scanning with `govulncheck`
- Automated dependency updates
- License compatibility checks
- Minimal dependency footprint

### Infrastructure
- Docker containers run as non-root
- Distroless base images for minimal attack surface
- Health checks for all services
- Graceful shutdown handling

## Security Checklist

### For Contributors
- [ ] No secrets in code or commits
- [ ] Input validation on all user data
- [ ] Proper error handling (no stack traces to users)
- [ ] Rate limiting considered
- [ ] Security headers in place
- [ ] Tests for security edge cases

### For Deployment
- [ ] HTTPS configured
- [ ] Secrets encrypted with SOPS
- [ ] Database credentials rotated
- [ ] Firewall rules configured
- [ ] Monitoring and alerting enabled
- [ ] Backup strategy in place

## Threat Model

### External Threats
1. **Injection Attacks**
   - Mitigation: Parameterized queries, input validation
   
2. **File Upload Abuse**
   - Mitigation: Type validation, size limits, sandboxed processing
   
3. **DoS/DDoS**
   - Mitigation: Rate limiting, resource limits, CDN/WAF
   
4. **Authentication Bypass**
   - Mitigation: Secure session management, CSRF protection

### Internal Threats
1. **Privilege Escalation**
   - Mitigation: Principle of least privilege, role-based access
   
2. **Data Leakage**
   - Mitigation: Structured logging, no PII in logs
   
3. **Supply Chain**
   - Mitigation: Dependency scanning, vendoring critical deps

## Incident Response

1. **Detection**: Monitoring, alerts, user reports
2. **Containment**: Isolate affected systems
3. **Eradication**: Remove threat, patch vulnerability
4. **Recovery**: Restore from backups if needed
5. **Lessons Learned**: Update security measures

## Compliance

- GDPR considerations for EU users
- Data retention policies
- Right to deletion support
- Audit logging capabilities

## Security Tools

```bash
# Vulnerability scanning
make vuln

# Static analysis
make lint

# Update dependencies
go get -u ./...
go mod tidy

# SOPS encryption
make encrypt-env
```

## Security Headers

Production deployments should include:
```
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Content-Security-Policy: default-src 'none'
Referrer-Policy: no-referrer
```

## Contact

For security concerns: security@example.com (update with actual contact)

## Acknowledgments

We appreciate responsible disclosure and will acknowledge security researchers who help improve our security.
