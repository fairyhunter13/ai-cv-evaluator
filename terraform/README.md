# Terraform Infrastructure

This directory contains Terraform configurations for managing infrastructure as code.

## Directory Structure

```
terraform/
├── cloudflare/           # Cloudflare DNS management
│   ├── main.tf          # Provider and DNS records
│   ├── variables.tf     # Input variables
│   ├── outputs.tf       # Output values
│   └── terraform.tfvars.example  # Example configuration
└── README.md            # This file
```

## Cloudflare DNS Management

### Prerequisites

1. **Terraform** installed (v1.0+)
   ```bash
   # macOS
   brew install terraform
   
   # Linux
   wget https://releases.hashicorp.com/terraform/1.6.0/terraform_1.6.0_linux_amd64.zip
   unzip terraform_1.6.0_linux_amd64.zip
   sudo mv terraform /usr/local/bin/
   ```

2. **Cloudflare API Token**
   - Go to https://dash.cloudflare.com/profile/api-tokens
   - Create token with **Zone:DNS:Edit** permissions for `ai-cv-evaluator.web.id`
   - Copy token value

3. **VPS Server IP**
   - Get IP from your VPS provider or `SSH_HOST` secret

### Setup

1. **Navigate to Cloudflare directory**:
   ```bash
   cd terraform/cloudflare
   ```

2. **Create `terraform.tfvars`** (copy from example):
   ```bash
   cp terraform.tfvars.example terraform.tfvars
   ```

3. **Edit `terraform.tfvars`** with your values:
   ```hcl
   cloudflare_api_token = "your-actual-cloudflare-api-token"
   server_ip            = "your-vps-ip-address"
   domain_name          = "ai-cv-evaluator.web.id"
   ```

4. **Initialize Terraform**:
   ```bash
   terraform init
   ```

### Usage

#### Preview DNS changes (safe, read-only)
```bash
terraform plan
```

This will show:
- **+** Resources to be created (auth.ai-cv-evaluator.web.id)
- **~** Resources to be modified
- **-** Resources to be destroyed (if any)

#### Apply DNS changes
```bash
terraform apply
```

Review the plan and type `yes` to confirm. Terraform will:
1. Create A record for `auth.ai-cv-evaluator.web.id` (Authelia)
2. Ensure `ai-cv-evaluator.web.id` and `dashboard.ai-cv-evaluator.web.id` exist
3. Output DNS configuration summary

#### Verify DNS records
```bash
# Check Terraform output
terraform output dns_summary

# Verify DNS resolution
dig auth.ai-cv-evaluator.web.id
dig dashboard.ai-cv-evaluator.web.id
dig ai-cv-evaluator.web.id
```

### DNS Records Managed

| Subdomain | Type | Target | Purpose |
|-----------|------|--------|---------|
| `@` (root) | A | VPS IP | Main application |
| `dashboard` | A | VPS IP | Admin dashboard |
| `auth` | A | VPS IP | **NEW** - Authelia SSO |

### Rollback

If you need to revert changes:

```bash
# Destroy all Terraform-managed DNS records
terraform destroy

# Then manually re-add records in Cloudflare dashboard
```

### Troubleshooting

#### "Error: Invalid Cloudflare API token"
- Check token permissions: Must have Zone:DNS:Edit for the zone
- Verify token in Cloudflare dashboard hasn't expired

#### "Error: could not find zone"
- Ensure `domain_name` in `terraform.tfvars` matches exactly
- Check domain is active in your Cloudflare account

#### DNS not resolving
- Wait 1-2 minutes for Cloudflare to propagate changes
- Check Cloudflare dashboard to verify records were created
- Use `terraform output` to see configured values

## Security Notes

- **Never commit `terraform.tfvars`** (contains API token) - already in `.gitignore`
- Store Terraform state securely (consider Terraform Cloud for team collaboration)
- Use least-privilege API tokens (Zone:DNS:Edit only, scoped to specific zone)

## Future Enhancements

- [ ] Terraform Cloud backend for remote state
- [ ] GitHub Actions integration for automated DNS updates
- [ ] Multi-environment support (staging, production)
- [ ] Additional Cloudflare features (WAF rules, page rules, SSL settings)
