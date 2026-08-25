---
name: terraform-infra
description: Use when ai-cv-evaluator infrastructure must change through Terraform, such as a Cloudflare DNS record or the VPS provisioning. Never change one of those by hand instead.
---

# Terraform Infrastructure Management

The host facts, the DNS table and the service list are in
[../references/production-inventory.md](../references/production-inventory.md). Bind `ORIGIN` and
`SSH` from it before the first command.

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
server_ip            = "<$ORIGIN>"
domain_name          = "example.com"
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

The table is in the inventory.

## VPS Provisioning

### Setup

```bash
cd terraform/vps
```

Variables (via `-var` or `terraform.tfvars`):

```hcl
server_ip       = "<$ORIGIN>"
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
terraform plan -var="server_ip=$ORIGIN" -var="ssh_user=ubuntu" -var="ssh_private_key=$(cat ~/.ssh/id_rsa)"
terraform apply -var="server_ip=$ORIGIN" -var="ssh_user=ubuntu" -var="ssh_private_key=$(cat ~/.ssh/id_rsa)"
```

## Secrets Management

- **SOPS encryption**: Secrets encrypted with AGE key
  `age1mxkhk7p4ngsl7yagkp0m2xa5ggzl2ppfgrfuadadsxdus8jcpugqsn9x5u`
- **Decrypt**: `sops -d secrets/env.production.sops.yaml`
- **Edit**: `sops secrets/env.production.sops.yaml`
- **Config**: `.sops.yaml` defines which files use which keys

## Gotchas

- **Never commit terraform.tfvars** — contains API tokens (already in .gitignore)
- **Terraform state**: Stored locally. Consider Terraform Cloud for team use.
- **fail2ban on VPS**: maxretry=3, findtime=600s, bantime=3600s. If banned, wait 1 hour or ask
  someone with console access to unban.
- **SOPS AGE key**: Must have the AGE private key in `$SOPS_AGE_KEY_FILE` or
  `~/.sops/age/keys.txt` to decrypt.