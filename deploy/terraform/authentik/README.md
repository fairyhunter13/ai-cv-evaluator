# Authentik Terraform - Secondary Admin Provisioning

This Terraform module creates a secondary Authentik admin user so you can log in with username/password from your existing environment (ADMIN_USERNAME/ADMIN_PASSWORD) instead of using the bootstrap email/password.

## Prerequisites

- Authentik is running and reachable
- An Authentik admin API token (`AK_TOKEN`) to allow Terraform to create users
- DNS/SSL configured for your Authentik endpoint

## Inputs

- `ak_url` (string): Authentik base URL, e.g. `https://dashboard.ai-cv-evaluator.web.id`
- `ak_token` (string, sensitive): Authentik API token with admin rights
- `ak_verify_ssl` (bool, default: true)
- `username` (string): Secondary admin username (e.g., `${ADMIN_USERNAME}`)
- `name` (string, default: "Admin"): Display name
- `email` (string): Email for the account (use `${ADMIN_USERNAME}@your-domain`)
- `password` (string, sensitive): Password for the account (e.g., `${ADMIN_PASSWORD}`)

## Usage

```bash
cd deploy/terraform/authentik

export TF_VAR_ak_url="https://dashboard.ai-cv-evaluator.web.id"
export TF_VAR_ak_token="$AK_TOKEN"  # Admin API token from Authentik
export TF_VAR_username="$ADMIN_USERNAME"
export TF_VAR_email="$ADMIN_USERNAME@your-domain"
export TF_VAR_password="$ADMIN_PASSWORD"

terraform init
terraform apply -auto-approve
```

After apply, you can log in to Authentik with the configured `username` and `password`.

## Notes

- You can rotate the token and password later in Authentik.
- Consider managing this user via code only (avoid manual edits) to keep state aligned.

