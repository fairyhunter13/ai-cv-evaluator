terraform {
  required_version = ">= 1.5.0"
  required_providers {
    authentik = {
      source  = "goauthentik/authentik"
      version = ">= 2024.6.0"
    }
  }
}

provider "authentik" {
  url        = var.ak_url
  token      = var.ak_token
  verify_ssl = var.ak_verify_ssl
}

resource "authentik_user" "secondary_admin" {
  username     = var.username
  name         = var.name
  email        = var.email
  is_active    = true
  is_superuser = true
  # Set password directly; ensure you rotate securely later
  password = var.password
}

output "secondary_admin_id" {
  value       = authentik_user.secondary_admin.id
  description = "ID of the created Authentik user"
}

