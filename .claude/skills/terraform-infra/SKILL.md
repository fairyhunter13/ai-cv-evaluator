---
name: terraform-infra
description: Manage ai-cv-evaluator infrastructure with Terraform (Cloudflare DNS, VPS provisioning)
---

# Terraform Infrastructure Management

## Directory Structure

```
terraform/
├── cloudflare/     # DNS record management
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   └── terraform.tfvars.example
└── vps/            # Server provisioning (Docker, fail2ban)
    └── main.tf
```

## Cloudflare DNS Management

### Setup

```bash
cd terraform/cloudflare
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:

```hcl
cloudflare_api_token = "<from .env.production CLOUDFLARE_API_TOKEN>"
server_ip            = "43.157.225.155"
domain_name          = "ai-cv-evaluator.web.id"
```

### Commands

```bash
cd terraform/cloudflare
terraform init          # First time only
terraform plan          # Preview changes (safe, read-only)
terraform apply         # Apply DNS changes
terraform output dns_summary  # Show current DNS config
terraform destroy       # Remove all managed DNS records (DANGEROUS)
```

### Managed DNS Records

| Subdomain   | Type | Target         | Purpose          |
| ----------- | ---- | -------------- | ---------------- |
| `@` (root)  | A    | 43.157.225.155 | Main application |
| `dashboard` | A    | 43.157.225.155 | Admin dashboard  |
| `auth`      | A    | 43.157.225.155 | Authelia SSO     |

## VPS Provisioning

### Setup

```bash
cd terraform/vps
```

Variables (via `-var` or `terraform.tfvars`):

```hcl
server_ip       = "43.157.225.155"
ssh_user        = "ubuntu"
ssh_private_key = file("~/.ssh/id_rsa")
```

### What It Does

1. **Docker install**: Installs Docker + Docker Compose if not present
2. **fail2ban install**: Configures SSH brute-force protection (maxretry=3, bantime=3600s)

### Commands

```bash
cd terraform/vps
terraform init
terraform plan -var="server_ip=43.157.225.155" -var="ssh_user=ubuntu" -var="ssh_private_key=$(cat ~/.ssh/id_rsa)"
terraform apply -var="server_ip=43.157.225.155" -var="ssh_user=ubuntu" -var="ssh_private_key=$(cat ~/.ssh/id_rsa)"
```

## Secrets Management

- **SOPS encryption**: Secrets encrypted with AGE key `age1mxkhk7p4ngsl7yagkp0m2xa5ggzl2ppfgrfuadadsxdus8jcpugqsn9x5u`
- **Decrypt**: `sops -d secrets/env.production.sops.yaml`
- **Edit**: `sops secrets/env.production.sops.yaml`
- **Config**: `.sops.yaml` defines which files use which keys

## Gotchas

- **Never commit terraform.tfvars** — contains API tokens (already in .gitignore)
- **Terraform state**: Stored locally. Consider Terraform Cloud for team use.
- **fail2ban on VPS**: maxretry=3, findtime=600s, bantime=3600s. If banned, wait 1 hour or ask someone with console access to unban.
- **SOPS AGE key**: Must have the AGE private key in `$SOPS_AGE_KEY_FILE` or `~/.sops/age/keys.txt` to decrypt.
