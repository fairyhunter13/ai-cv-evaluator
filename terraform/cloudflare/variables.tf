variable "cloudflare_api_token" {
  description = "Cloudflare API token with Zone:DNS:Edit permissions"
  type        = string
  sensitive   = true
}

variable "domain_name" {
  description = "Root domain name managed in Cloudflare"
  type        = string
  default     = "ai-cv-evaluator.web.id"
}

variable "server_ip" {
  description = "VPS server IP address for A records"
  type        = string
}
