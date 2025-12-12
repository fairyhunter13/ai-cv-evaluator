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
  zone_id = data.cloudflare_zone.main.id
  name    = "@"
  type    = "A"
  value   = var.server_ip
  proxied = true  # Enable Cloudflare proxy for DDoS protection, SSL, and caching
  ttl     = 1     # Auto when proxied
  comment = "Main application domain"
}

# Dashboard subdomain
resource "cloudflare_record" "dashboard" {
  zone_id = data.cloudflare_zone.main.id
  name    = "dashboard"
  type    = "A"
  value   = var.server_ip
  proxied = true
  ttl     = 1
  comment = "Admin dashboard subdomain"
}

# Authelia SSO subdomain (NEW - replaces Keycloak)
resource "cloudflare_record" "auth" {
  zone_id = data.cloudflare_zone.main.id
  name    = "auth"
  type    = "A"
  value   = var.server_ip
  proxied = true
  ttl     = 1
  comment = "Authelia SSO authentication subdomain"
}

# NOTE: keycloak.ai-cv-evaluator.web.id intentionally removed
# Migration from Keycloak to Authelia completed
