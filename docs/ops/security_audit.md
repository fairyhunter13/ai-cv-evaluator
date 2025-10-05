# Security Audit Procedures

This document outlines comprehensive security audit procedures for the AI CV Evaluator project, including automated checks, manual reviews, and incident response protocols.

## Overview

The security audit process ensures the application maintains high security standards through regular assessments, vulnerability scanning, and compliance verification.

## Security Audit Framework

### 1. Automated Security Checks

#### 1.1 Dependency Vulnerability Scanning
```bash
# Run vulnerability scanning
make vuln

# Check for known vulnerabilities
govulncheck ./...

# Update dependencies
go get -u ./...
go mod tidy
```

#### 1.2 Static Code Analysis
```bash
# Run security-focused linting
golangci-lint run --enable=gosec ./...

# Run gosec security scanner
make gosec-sarif

# Check for security anti-patterns
golangci-lint run --enable=govet,ineffassign,misspell ./...
```

#### 1.3 Container Security Scanning
```bash
# Scan Docker images for vulnerabilities
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  aquasec/trivy image ghcr.io/fairyhunter13/ai-cv-evaluator:latest

# Scan for secrets in containers
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  aquasec/trivy image --security-checks secret \
  ghcr.io/fairyhunter13/ai-cv-evaluator:latest
```

### 2. Manual Security Review

#### 2.1 Code Review Checklist

**Authentication & Authorization**
- [ ] Session management is secure (HttpOnly, Secure, SameSite)
- [ ] No hardcoded credentials in code
- [ ] Proper input validation on all endpoints
- [ ] Rate limiting implemented on sensitive endpoints
- [ ] CSRF protection where applicable

**Input Validation**
- [ ] File upload validation (type, size, content)
- [ ] JSON schema validation on API endpoints
- [ ] SQL injection prevention (parameterized queries)
- [ ] XSS prevention (proper output encoding)
- [ ] Path traversal prevention

**Secrets Management**
- [ ] No secrets in code or configuration files
- [ ] SOPS encryption for sensitive data
- [ ] Environment-specific secrets
- [ ] Secrets rotation procedures documented
- [ ] Access logging for sensitive operations

**Network Security**
- [ ] CORS configuration with allowlisting
- [ ] Security headers implemented
- [ ] HTTPS enforcement in production
- [ ] Internal service communication secured
- [ ] Firewall rules documented

#### 2.2 Infrastructure Security Review

**Container Security**
- [ ] Non-root user in containers
- [ ] Minimal base images (distroless)
- [ ] No unnecessary packages installed
- [ ] Health checks implemented
- [ ] Resource limits configured

**Database Security**
- [ ] Database credentials encrypted
- [ ] Connection encryption (SSL/TLS)
- [ ] Database access logging
- [ ] Backup encryption
- [ ] Regular security updates

**Queue Security**
- [ ] Redpanda/Kafka security configuration
- [ ] Message encryption in transit
- [ ] Access control on topics
- [ ] Audit logging enabled
- [ ] Network isolation

### 3. Security Testing

#### 3.1 Penetration Testing

**API Security Testing**
```bash
# Test for common vulnerabilities
# SQL injection
curl -X POST "http://localhost:8080/v1/evaluate" \
  -H "Content-Type: application/json" \
  -d '{"cv_id": "1'\'' OR 1=1--", "project_id": "test"}'

# XSS testing
curl -X POST "http://localhost:8080/v1/evaluate" \
  -H "Content-Type: application/json" \
  -d '{"cv_id": "<script>alert(1)</script>", "project_id": "test"}'

# File upload testing
curl -X POST "http://localhost:8080/v1/upload" \
  -F "cv=@malicious.php" \
  -F "project=@test.txt"
```

**Authentication Testing**
```bash
# Test session security
curl -v -c cookies.txt -b cookies.txt \
  -X POST "http://localhost:8080/admin/login" \
  -d "username=admin&password=test"

# Test rate limiting
for i in {1..100}; do
  curl -X POST "http://localhost:8080/v1/evaluate" \
    -H "Content-Type: application/json" \
    -d '{"cv_id": "test", "project_id": "test"}'
done
```

#### 3.2 Security Headers Testing

```bash
# Check security headers
curl -I http://localhost:8080/healthz

# Expected headers:
# X-Content-Type-Options: nosniff
# X-Frame-Options: DENY
# X-XSS-Protection: 1; mode=block
# Strict-Transport-Security: max-age=31536000
```

### 4. Compliance Auditing

#### 4.1 GDPR Compliance

**Data Protection**
- [ ] Personal data identification and mapping
- [ ] Data retention policies implemented
- [ ] Right to deletion procedures
- [ ] Data processing consent mechanisms
- [ ] Privacy impact assessments

**Data Minimization**
- [ ] Only necessary data collected
- [ ] Data anonymization where possible
- [ ] Regular data cleanup procedures
- [ ] Data access logging
- [ ] Data breach notification procedures

#### 4.2 Security Standards

**OWASP Top 10 Compliance**
- [ ] A01: Broken Access Control
- [ ] A02: Cryptographic Failures
- [ ] A03: Injection
- [ ] A04: Insecure Design
- [ ] A05: Security Misconfiguration
- [ ] A06: Vulnerable Components
- [ ] A07: Authentication Failures
- [ ] A08: Software Integrity Failures
- [ ] A09: Logging Failures
- [ ] A10: Server-Side Request Forgery

### 5. Security Monitoring

#### 5.1 Automated Monitoring

**Security Metrics**
```yaml
# Prometheus metrics for security monitoring
security_audit_failures_total
authentication_failures_total
authorization_failures_total
input_validation_failures_total
file_upload_attempts_total
rate_limit_violations_total
```

**Alerting Rules**
```yaml
# Grafana alerting rules
- alert: HighAuthenticationFailures
  expr: rate(authentication_failures_total[5m]) > 10
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "High authentication failure rate"

- alert: SuspiciousFileUploads
  expr: rate(file_upload_attempts_total[5m]) > 5
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Suspicious file upload activity"
```

#### 5.2 Log Analysis

**Security Event Logging**
```go
// Example security event logging
func logSecurityEvent(event string, details map[string]interface{}) {
    slog.Info("security_event",
        slog.String("event", event),
        slog.String("timestamp", time.Now().UTC().Format(time.RFC3339)),
        slog.Any("details", details),
        slog.String("source_ip", getClientIP()),
        slog.String("user_agent", getUserAgent()),
    )
}
```

**Log Analysis Queries**
```bash
# Find authentication failures
grep "authentication_failure" /var/log/app.log | tail -100

# Find suspicious file uploads
grep "file_upload" /var/log/app.log | grep -v "success" | tail -50

# Find rate limit violations
grep "rate_limit" /var/log/app.log | tail -20
```

### 6. Security Audit Schedule

#### 6.1 Daily Checks
- [ ] Automated vulnerability scans
- [ ] Security log review
- [ ] Failed authentication attempts
- [ ] Unusual traffic patterns
- [ ] System resource usage

#### 6.2 Weekly Checks
- [ ] Dependency updates
- [ ] Security patch review
- [ ] Access log analysis
- [ ] Configuration drift check
- [ ] Backup verification

#### 6.3 Monthly Checks
- [ ] Full security scan
- [ ] Penetration testing
- [ ] Compliance review
- [ ] Security training
- [ ] Incident response drill

#### 6.4 Quarterly Checks
- [ ] Comprehensive security audit
- [ ] Third-party security assessment
- [ ] Security policy review
- [ ] Disaster recovery testing
- [ ] Security architecture review

### 7. Security Audit Tools

#### 7.1 Automated Tools
```bash
# Install security tools
make tools

# Run security scans
make vuln
make gosec-sarif
make license-scan
```

#### 7.2 Manual Testing Tools
```bash
# OWASP ZAP for web application testing
docker run -t owasp/zap2docker-stable zap-baseline.py -t http://localhost:8080

# Nmap for network scanning
nmap -sV -sC localhost

# Nikto for web server testing
nikto -h http://localhost:8080
```

### 8. Security Audit Report Template

#### 8.1 Executive Summary
- Overall security posture
- Critical vulnerabilities found
- Compliance status
- Recommendations

#### 8.2 Technical Findings
- Vulnerability details
- Risk assessment
- Remediation steps
- Timeline for fixes

#### 8.3 Compliance Status
- GDPR compliance
- OWASP Top 10 status
- Security standards adherence
- Regulatory requirements

### 9. Remediation Procedures

#### 9.1 Critical Vulnerabilities
1. **Immediate Action**: Patch within 24 hours
2. **Documentation**: Update security procedures
3. **Testing**: Verify fix effectiveness
4. **Monitoring**: Enhanced monitoring for 48 hours

#### 9.2 High-Risk Issues
1. **Action**: Patch within 1 week
2. **Documentation**: Update procedures
3. **Testing**: Comprehensive testing
4. **Monitoring**: Monitor for 1 week

#### 9.3 Medium-Risk Issues
1. **Action**: Patch within 1 month
2. **Documentation**: Update procedures
3. **Testing**: Standard testing
4. **Monitoring**: Regular monitoring

### 10. Security Audit Checklist

#### 10.1 Pre-Audit Preparation
- [ ] Security tools updated
- [ ] Test environment prepared
- [ ] Documentation reviewed
- [ ] Stakeholders notified
- [ ] Backup procedures verified

#### 10.2 During Audit
- [ ] Automated scans completed
- [ ] Manual testing performed
- [ ] Documentation reviewed
- [ ] Interviews conducted
- [ ] Evidence collected

#### 10.3 Post-Audit
- [ ] Findings documented
- [ ] Risk assessment completed
- [ ] Remediation plan created
- [ ] Stakeholders notified
- [ ] Follow-up scheduled

## Security Audit Automation

### GitHub Actions Security Workflow
```yaml
name: Security Audit
on:
  schedule:
    - cron: '0 2 * * 1'  # Weekly on Monday at 2 AM
  workflow_dispatch:

jobs:
  security-audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run security scans
        run: |
          make vuln
          make gosec-sarif
          make license-scan
      - name: Upload security reports
        uses: actions/upload-artifact@v3
        with:
          name: security-reports
          path: |
            gosec-results.sarif
            fossa-results.json
```

## Contact and Escalation

### Security Issues
- **Email**: security@example.com
- **Slack**: #security-alerts
- **Phone**: +1-XXX-XXX-XXXX (24/7)

### Escalation Matrix
1. **Level 1**: Development team (immediate)
2. **Level 2**: Security team (within 1 hour)
3. **Level 3**: Management (within 4 hours)
4. **Level 4**: External security (within 24 hours)

---

*This security audit framework ensures comprehensive security assessment and continuous improvement of the AI CV Evaluator project's security posture.*
