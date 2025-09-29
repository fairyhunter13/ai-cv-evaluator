# GitHub Actions Secrets Reference

This document lists the secrets expected by our GitHub Actions workflows. Do NOT commit secret values to the repo. Add them under the repository or organization Settings → Secrets and variables → Actions.

Each secret name is case-sensitive.

## Required secrets (CI/CD)
- GITHUB_TOKEN
  - Purpose: Default token for authenticating to GHCR and GitHub APIs during CI/CD.
  - Used by: `.github/workflows/ci.yml` and `.github/workflows/deploy.yml` (login/push to GHCR, repo operations)
- OPENROUTER_API_KEY
  - Purpose: Real chat completions for live E2E on tag releases (pre-deploy gate).
  - Used by: `.github/workflows/deploy.yml` (e2e-verify job only)
- OPENAI_API_KEY
  - Purpose: Real embeddings for RAG/evaluation (pre-deploy gate).
  - Used by: `.github/workflows/deploy.yml` (e2e-verify job only)
- SOPS_AGE_KEY
  - Purpose: Age private key content for decrypting `.sops` files in CI/CD (e.g., `.env.production.sops.yaml`).
  - Used by: `.github/workflows/ci.yml` (if decrypting CI env) and `.github/workflows/deploy.yml` (materialized to `~/.config/sops/age/keys.txt`)
- SSH_PRIVATE_KEY
  - Purpose: Private key for SSH to the deployment host (VPS). Stored as a multi-line secret (PEM).
  - Used by: `.github/workflows/deploy.yml` (written to `~/.ssh/id_rsa` on runner)
- SSH_HOST
  - Purpose: Target host (e.g., `1.2.3.4`).
  - Used by: `.github/workflows/deploy.yml`
- SSH_USER
  - Purpose: SSH username (e.g., `ubuntu`).
  - Used by: `.github/workflows/deploy.yml`

## TLS certificates (optional)
- LETSENCRYPT_EMAIL
  - Purpose: Contact email for Let's Encrypt certificate issuance/renewal.
  - Used by: `.github/workflows/deploy.yml` (cert issuance), `.github/workflows/renew-cert.yml` (renewal)

#### Notes
- CI steps are routed through the Makefile (lint, vet, vuln, tests, OpenAPI validation, build matrix) to keep a single source of truth for scripts.
- Live E2E runs only in the deploy workflow as a pre-deploy gate. They require `OPENROUTER_API_KEY` and `OPENAI_API_KEY`.
- The chat model is optional; if not provided, the app defaults to `openrouter/auto`. You may set `CHAT_FALLBACK_MODELS` via runtime env to specify fallbacks.
- Optional scanners (Snyk, Semgrep, FOSSA) are documented in `github-optional-secrets.md`.
