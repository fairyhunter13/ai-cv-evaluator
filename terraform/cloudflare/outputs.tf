output "zone_id" {
  description = "Cloudflare zone ID"
  value       = data.cloudflare_zone.main.id
}

output "dns_records" {
  description = "Configured DNS records"
  value = {
    app = {
      hostname = cloudflare_record.app.hostname
      content  = cloudflare_record.app.content
    }
    dashboard = {
      hostname = cloudflare_record.dashboard.hostname
      content  = cloudflare_record.dashboard.content
    }
    auth = {
      hostname = cloudflare_record.auth.hostname
      content  = cloudflare_record.auth.content
    }
  }
}

output "dns_summary" {
  description = "DNS configuration summary"
  value = <<-EOT
    Cloudflare DNS Configuration:
    - Main App:     ${cloudflare_record.app.hostname} -> ${cloudflare_record.app.value}
    - Dashboard:    ${cloudflare_record.dashboard.hostname} -> ${cloudflare_record.dashboard.value}
    - Auth (NEW):   ${cloudflare_record.auth.hostname} -> ${cloudflare_record.auth.value}
    
    Note: keycloak.ai-cv-evaluator.web.id removed (migrated to Authelia)
  EOT
}
