# GitHub Actions Secrets Management

This document provides comprehensive guidance for managing GitHub Actions secrets for the AI CV Evaluator project.

## 🎯 Overview

GitHub Actions secrets are used to securely store sensitive information required for CI/CD pipelines, deployment, and security scanning. This guide covers both required and optional secrets.

## 🔐 Required Secrets (CI/CD)

### Core CI/CD Secrets

#### GITHUB_TOKEN
- **Purpose**: Default token for authenticating to GHCR and GitHub APIs during CI/CD
- **Used by**: `.github/workflows/ci.yml` and `.github/workflows/deploy.yml`
- **Scope**: Login/push to GHCR, repository operations
- **Auto-generated**: Yes, provided by GitHub Actions

#### OPENROUTER_API_KEY
- **Purpose**: Real chat completions for live E2E on tag releases (pre-deploy gate)
- **Used by**: `.github/workflows/deploy.yml` (e2e-verify job only)
- **How to obtain**: 
  1. Sign up at [OpenRouter](https://openrouter.ai/)
  2. Generate API key from dashboard
  3. Add to repository secrets

#### OPENAI_API_KEY
- **Purpose**: Real embeddings for RAG/evaluation (pre-deploy gate)
- **Used by**: `.github/workflows/deploy.yml` (e2e-verify job only)
- **How to obtain**:
  1. Create account at [OpenAI](https://platform.openai.com/)
  2. Generate API key from API section
  3. Add to repository secrets

#### SOPS_AGE_KEY
- **Purpose**: Age private key content for decrypting `.sops` files in CI/CD
- **Used by**: `.github/workflows/ci.yml` and `.github/workflows/deploy.yml`
- **How to obtain**:
  ```bash
  # Generate age key pair
  age-keygen -o age-key.txt
  
  # Extract private key content
  cat age-key.txt
  ```
- **Note**: Store the private key content as the secret value

#### SSH_PRIVATE_KEY
- **Purpose**: Private key for SSH to the deployment host (VPS)
- **Used by**: `.github/workflows/deploy.yml`
- **Format**: Multi-line PEM format
- **How to obtain**:
  ```bash
  # Generate SSH key pair
  ssh-keygen -t rsa -b 4096 -C "deployment@ai-cv-evaluator"
  
  # Copy private key content
  cat ~/.ssh/id_rsa
  ```

#### SSH_HOST
- **Purpose**: Target deployment host IP or hostname
- **Used by**: `.github/workflows/deploy.yml`
- **Example**: `1.2.3.4` or `deploy.example.com`
- **Note**: Use IP address for better reliability

#### SSH_USER
- **Purpose**: SSH username for deployment host
- **Used by**: `.github/workflows/deploy.yml`
- **Example**: `ubuntu`, `deploy`, `root`
- **Note**: Use non-root user for security

## 🔧 Optional Secrets (Security & Compliance)

### Security Scanning

#### SEMGREP_APP_TOKEN
- **Purpose**: Authentication for Semgrep Pro rules
- **Used by**: `.github/workflows/security.yml`
- **How to obtain**:
  1. Sign in to [Semgrep AppSec Platform](https://semgrep.dev/)
  2. Navigate to account or org settings
  3. Generate App Token
  4. Add to repository secrets
- **Documentation**: [Semgrep API](https://semgrep.dev/docs/semgrep-appsec-platform/semgrep-api)

#### SNYK_TOKEN
- **Purpose**: Snyk vulnerability scanning
- **Used by**: `.github/workflows/security.yml`
- **How to obtain**:
  1. Create account at [Snyk](https://snyk.io/)
  2. Run `snyk auth` locally or copy API token from Account settings
  3. Add to repository secrets
- **Documentation**: [Snyk CLI Authentication](https://docs.snyk.io/snyk-cli/commands/auth)

#### FOSSA_API_KEY
- **Purpose**: License compliance analysis upload
- **Used by**: `.github/workflows/ci.yml`
- **How to obtain**:
  1. Create account at [FOSSA](https://fossa.com/)
  2. Navigate to Account Settings → Integrations → API
  3. Generate API key
  4. Add to repository secrets
- **Documentation**: [FOSSA Authentication](https://docs.fossa.com/docs/authentication)

### TLS Certificates (Optional)

#### LETSENCRYPT_EMAIL
- **Purpose**: Contact email for Let's Encrypt certificate issuance/renewal
- **Used by**: `.github/workflows/deploy.yml` and `.github/workflows/renew-cert.yml`
- **Format**: Valid email address
- **Example**: `admin@example.com`
- **Note**: Used for certificate notifications and renewal

## 🚀 Secret Configuration

### Adding Secrets to Repository

1. **Navigate to Repository Settings**
   - Go to your repository on GitHub
   - Click "Settings" tab
   - Select "Secrets and variables" → "Actions"

2. **Add New Secret**
   - Click "New repository secret"
   - Enter secret name (case-sensitive)
   - Enter secret value
   - Click "Add secret"

3. **Verify Secret**
   - Check that secret appears in the list
   - Ensure name matches exactly (case-sensitive)
   - Verify value is correct

### Secret Naming Conventions

- **Use UPPERCASE** for all secret names
- **Use underscores** to separate words
- **Be descriptive** but concise
- **Follow existing patterns** for consistency

### Security Best Practices

#### Secret Value Guidelines
- **Use strong, unique values** for all secrets
- **Rotate secrets regularly** (every 90 days)
- **Never commit secrets** to repository
- **Use environment-specific values** when possible

#### Access Control
- **Limit access** to repository settings
- **Use organization secrets** for shared values
- **Audit secret usage** regularly
- **Remove unused secrets** promptly

## 🔄 Secret Rotation

### Automated Rotation
- **API Keys**: Rotate every 90 days
- **SSH Keys**: Rotate every 180 days
- **Tokens**: Rotate every 60 days
- **Certificates**: Auto-renewed by Let's Encrypt

### Manual Rotation Process
1. **Generate new secret** using appropriate method
2. **Update repository secret** with new value
3. **Test deployment** to verify new secret works
4. **Remove old secret** after verification
5. **Update documentation** if needed

## 📊 Secret Monitoring

### Usage Tracking
- **Monitor secret usage** in workflow logs
- **Track secret access** patterns
- **Alert on unusual usage** patterns
- **Log secret operations** for audit

### Health Checks
- **Verify secret validity** before deployment
- **Test secret functionality** in CI/CD
- **Monitor secret expiration** dates
- **Alert on secret failures**

## 🛡️ Security Considerations

### Secret Storage
- **GitHub encrypts secrets** at rest
- **Secrets are masked** in logs
- **Access is logged** for audit
- **Secrets are environment-specific**

### Secret Access
- **Limit repository access** to trusted users
- **Use least privilege** principle
- **Monitor access patterns** regularly
- **Revoke access** when no longer needed

### Secret Lifecycle
- **Create secrets** with strong values
- **Use secrets** only when needed
- **Rotate secrets** regularly
- **Remove secrets** when obsolete

## ✅ Definition of Done (Secrets)

### Required Secrets
- **All required secrets** are configured
- **Secrets have valid values** and work correctly
- **Access is properly restricted** to authorized users
- **Documentation is updated** with current secret names

### Optional Secrets
- **Security scanning secrets** are configured if needed
- **TLS certificate secrets** are configured if using Let's Encrypt
- **All secrets are properly documented** with setup instructions

### Security Requirements
- **No secrets are committed** to repository
- **Secrets are rotated** according to schedule
- **Access is audited** regularly
- **Unused secrets are removed** promptly

This document serves as the comprehensive guide for managing GitHub Actions secrets securely and effectively for the AI CV Evaluator project.
