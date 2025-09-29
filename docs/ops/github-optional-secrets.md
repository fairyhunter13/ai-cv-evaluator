# GitHub Actions Optional Secrets

These secrets are optional. If present, additional security scans and notifications are enabled.

- SEMGREP_APP_TOKEN
  - Purpose: Auth for Semgrep Pro rules.
  - Used by: `.github/workflows/security.yml`
  - How to obtain: Sign in to Semgrep AppSec Platform, generate an App Token from account or org settings, and set it as a repo secret. See: https://semgrep.dev/docs/semgrep-appsec-platform/semgrep-api and https://semgrep.dev/docs/deployment/add-semgrep-to-ci
- SNYK_TOKEN
  - Purpose: Snyk scan auth.
  - Used by: `.github/workflows/security.yml`
  - How to obtain: From Snyk, run `snyk auth` locally or copy your API token from Account settings. Docs: https://docs.snyk.io/snyk-cli/commands/auth and https://docs.snyk.io/snyk-cli/authenticate-to-use-the-cli
- FOSSA_API_KEY
  - Purpose: License compliance analysis upload.
  - Used by: `.github/workflows/ci.yml`
  - How to obtain: Create an account at FOSSA and generate an API key in Account Settings → Integrations → API. Docs: https://docs.fossa.com/docs/authentication

- Notes: Where applicable, CI steps are Makefile-driven; scanners/actions that require their own GitHub Action remain unaffected.
