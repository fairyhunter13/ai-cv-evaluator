# GitHub Actions Optional Secrets

These secrets are optional. If present, additional security scans and notifications are enabled.

- SEMGREP_APP_TOKEN
  - Purpose: Auth for Semgrep Pro rules.
  - Used by: `.github/workflows/security.yml`
- SNYK_TOKEN
  - Purpose: Snyk scan auth.
  - Used by: `.github/workflows/security.yml`
- FOSSA_API_KEY
  - Purpose: License compliance analysis upload.
  - Used by: `.github/workflows/ci.yml`
- SLACK_WEBHOOK_URL
  - Purpose: Deployment notifications to Slack.
  - Used by: `.github/workflows/deploy.yml`
