variable "ak_url" {
  description = "Authentik base URL (e.g., https://dashboard.ai-cv-evaluator.web.id)"
  type        = string
}

variable "ak_token" {
  description = "Authentik API token with admin permissions"
  type        = string
  sensitive   = true
}

variable "ak_verify_ssl" {
  description = "Verify SSL when talking to Authentik"
  type        = bool
  default     = true
}

variable "username" {
  description = "Secondary admin username"
  type        = string
}

variable "name" {
  description = "Display name for the user"
  type        = string
  default     = "Admin"
}

variable "email" {
  description = "Email for the user"
  type        = string
}

variable "password" {
  description = "Password for the user"
  type        = string
  sensitive   = true
}

