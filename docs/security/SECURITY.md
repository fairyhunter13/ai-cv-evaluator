# Security and Compliance

This document outlines the comprehensive security measures, compliance requirements, and best practices for the AI CV Evaluator service.

## 🎯 Security Overview

The security framework ensures:
- **Input validation** and sanitization
- **Secure data handling** and storage
- **Authentication** and authorization
- **Network security** and encryption
- **Compliance** with security standards

## 🔒 Input Security

### File Upload Security
- **Allowlist approach**: Only allow `.txt`, `.pdf`, `.docx`
- **Content sniffing**: Detect MIME type by content, not extension
- **Size limits**: 10MB per file (configurable)
- **Virus scanning**: Optional integration with ClamAV
- **Content sanitization**: Strip control characters and malicious content

### Input Validation
```go
// File type validation
func validateFileType(filename string, content []byte) error {
    // Check extension
    ext := strings.ToLower(filepath.Ext(filename))
    if !contains(allowedExtensions, ext) {
        return ErrUnsupportedFileType
    }
    
    // Check MIME type by content
    mimeType := http.DetectContentType(content)
    if !contains(allowedMimeTypes, mimeType) {
        return ErrUnsupportedMimeType
    }
    
    return nil
}
```

## 🔐 Authentication and Authorization

### Admin Authentication
- **Username/password** authentication
- **Argon2id** password hashing
- **Session management** with secure cookies
- **CSRF protection** for form submissions
- **Login throttling** to prevent brute force

### Password Security
```go
// Argon2id password hashing
func HashPassword(password string) (string, error) {
    salt := make([]byte, 16)
    if _, err := rand.Read(salt); err != nil {
        return "", err
    }
    
    hash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
    
    return fmt.Sprintf("argon2id$%d$%d$%d$%s$%s",
        3, 64*1024, 2,
        base64.RawStdEncoding.EncodeToString(salt),
        base64.RawStdEncoding.EncodeToString(hash)), nil
}
```

## 🌐 Network Security

### HTTP Security Headers
```go
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Prevent MIME type sniffing
        w.Header().Set("X-Content-Type-Options", "nosniff")
        
        // Prevent clickjacking
        w.Header().Set("X-Frame-Options", "DENY")
        
        // Content Security Policy
        w.Header().Set("Content-Security-Policy", "default-src 'none'")
        
        // Referrer policy
        w.Header().Set("Referrer-Policy", "no-referrer")
        
        // HSTS (HTTPS only)
        if r.TLS != nil {
            w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
        }
        
        next.ServeHTTP(w, r)
    })
}
```

## 🔐 Secrets Management

### Environment Variables
```bash
# Production secrets (never commit)
OPENROUTER_API_KEY=your-api-key
OPENAI_API_KEY=your-api-key
DB_PASSWORD=secure-password
SESSION_SECRET=random-secret-key
CSRF_SECRET=random-csrf-key
```

### SOPS Encryption
```bash
# Encrypt secrets file
sops -e -i .env.prod

# Decrypt for deployment
sops -d .env.prod.sops > .env.prod
```

## 🛡️ Data Protection

### Encryption at Rest
- **Database encryption** via PostgreSQL
- **Vector store encryption** via Qdrant
- **File system encryption** on VPS
- **Backup encryption** for data retention

### Encryption in Transit
- **TLS 1.2+** for all HTTP connections
- **Database connections** via SSL
- **API communications** via HTTPS
- **Internal service** communication secured

## 🔍 Security Monitoring

### Audit Logging
```go
// Security event logging
func LogSecurityEvent(ctx context.Context, event SecurityEvent) {
    logger := slog.FromContext(ctx)
    logger.Info("security_event",
        slog.String("event_type", event.Type),
        slog.String("user_id", event.UserID),
        slog.String("ip_address", event.IPAddress),
        slog.String("user_agent", event.UserAgent),
        slog.Any("details", event.Details),
    )
}
```

### Vulnerability Scanning
```bash
# Container vulnerability scanning
trivy image ghcr.io/owner/ai-cv-evaluator:latest

# Dependency vulnerability scanning
govulncheck ./...

# SAST scanning
golangci-lint run --enable gosec ./...
```

## 📋 Compliance Requirements

### Data Privacy
- **GDPR compliance** for EU users
- **Data minimization** principles
- **Right to deletion** implementation
- **Data portability** support

### Security Standards
- **OWASP Top 10** compliance
- **CIS benchmarks** implementation
- **Security headers** validation
- **Input validation** comprehensive

## 🚨 Incident Response

### Security Incident Playbook
1. **Detection**: Automated monitoring and alerting
2. **Assessment**: Severity and impact analysis
3. **Containment**: Isolate affected systems
4. **Investigation**: Root cause analysis
5. **Recovery**: Restore normal operations
6. **Lessons learned**: Process improvement

## 📊 Security Metrics

### Key Performance Indicators
- **Vulnerability count** by severity
- **Security incidents** per month
- **Mean time to detection** (MTTD)
- **Mean time to resolution** (MTTR)

## ✅ Definition of Done (Security)

### Implementation Requirements
- **Security headers** implemented
- **Input validation** comprehensive
- **Authentication** working
- **Encryption** in place
- **Monitoring** active

### Compliance Requirements
- **OWASP Top 10** addressed
- **Security standards** met
- **Audit logging** functional
- **Incident response** tested

## 🔒 Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## 🚨 Reporting a Vulnerability

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
