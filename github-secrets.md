# GitHub Actions Secrets Reference

This document lists the secrets expected by our GitHub Actions workflows. Do NOT commit secret values to the repo. Add them under the repository or organization Settings → Secrets and variables → Actions.

Each secret name is case-sensitive.

## Core build/test (CI)
- OPENROUTER_API_KEY
  - Purpose: Real chat completions for live E2E on tag releases.
  - Used by: `.github/workflows/ci.yml` (tags only), `.github/workflows/deploy.yml` (e2e-verify gate)
- OPENAI_API_KEY
  - Purpose: Real embeddings for RAG and evaluation pipeline.
  - Used by: `.github/workflows/ci.yml` (tags only), `.github/workflows/deploy.yml` (e2e-verify gate)

## SOPS (for decrypting encrypted files during deploy)
- SOPS_AGE_KEY
  - Purpose: Age private key content for decrypting `.sops` files in CI/CD (e.g., `.env.production.sops.yaml`).
  - Used by: `.github/workflows/deploy.yml` (materialized to `~/.config/sops/age/keys.txt`)

## Deployment (SSH)
- SSH_PRIVATE_KEY
  - Purpose: Private key for SSH to the deployment host (VPS). Stored as a multi-line secret (PEM).
  - Used by: `.github/workflows/deploy.yml` (written to `~/.ssh/id_rsa` on runner)
- SSH_HOST
  - Purpose: Target host (e.g., `1.2.3.4`).
  - Used by: `.github/workflows/deploy.yml`
- SSH_USER
  - Purpose: SSH username (e.g., `ubuntu`).
  - Used by: `.github/workflows/deploy.yml`

## TLS Certificates (optional, for Certbot in deploy/renew workflows)
- LETSENCRYPT_EMAIL
  - Purpose: Contact email for Let's Encrypt certificate issuance/renewal.
  - Used by: `.github/workflows/deploy.yml` (cert issuance), `.github/workflows/renew-cert.yml` (renewal)

## Notes
- E2E-on-tags only: CI runs the E2E suite exclusively on tag releases to keep PR and main branch CI fast.
- Live E2E requires `OPENROUTER_API_KEY` and `OPENAI_API_KEY`.
- The chat model is optional; if not provided, the app defaults to `openrouter/auto`. You may set `CHAT_FALLBACK_MODELS` via runtime env to specify fallbacks.
- Optional scanners/notifications (e.g., Snyk, Semgrep, FOSSA, Slack) are documented in `github-optional-secrets.md`.
