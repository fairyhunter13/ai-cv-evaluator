terraform {
  required_version = ">= 1.0"
  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.0"
    }
  }
  
  # Terraform state stored locally by default
  # For team collaboration, consider:
  # backend "remote" {
  #   organization = "your-org"
  #   workspaces {
  #     name = "ai-cv-evaluator-dns"
  #   }
  # }
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}

data "cloudflare_zone" "main" {
  name = var.domain_name
}

# Main application domain
resource "cloudflare_record" "app" {
  zone_id        = data.cloudflare_zone.main.id
  name           = "@"
  type           = "A"
  content        = var.server_ip
  proxied        = true  # Enable Cloudflare proxy for DDoS protection, SSL, and caching
  ttl            = 1     # Auto when proxied
  comment        = "Main application domain"
  allow_overwrite = true  # Allow updating existing records
}

# Dashboard subdomain
resource "cloudflare_record" "dashboard" {
  zone_id        = data.cloudflare_zone.main.id
  name           = "dashboard"
  type           = "A"
  content        = var.server_ip
  proxied        = true
  ttl            = 1
  comment        = "Admin dashboard subdomain"
  allow_overwrite = true  # Allow updating existing records
}

# Authelia SSO subdomain
resource "cloudflare_record" "auth" {
  zone_id        = data.cloudflare_zone.main.id
  name           = "auth"
  type           = "A"
  content        = var.server_ip
  proxied        = true
  ttl            = 1
  comment        = "Authelia SSO authentication subdomain"
  allow_overwrite = true  # Allow updating existing records
}
