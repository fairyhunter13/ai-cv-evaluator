terraform {
  required_version = ">= 1.0"
  # This configuration assumes you are provisioning a server that already exists (BYO)
  # or you can adapt it to create a Droplet/EC2 instance.
}

variable "server_ip" {
  description = "IP address of the VPS to provision"
  type        = string
}

variable "ssh_user" {
  description = "SSH user (usually root)"
  type        = string
  default     = "root"
}

variable "ssh_private_key" {
  description = "SSH private key for connection"
  type        = string
  sensitive   = true
}

# Provisioner to install Docker and Docker Compose
resource "null_resource" "docker_install" {
  triggers = {
    # Run this if the server IP changes
    server_ip = var.server_ip
  }

  connection {
    type        = "ssh"
    user        = var.ssh_user
    private_key = var.ssh_private_key
    host        = var.server_ip
    timeout     = "2m"
  }

  provisioner "remote-exec" {
    inline = [
      "export DEBIAN_FRONTEND=noninteractive",
      # Check if Docker is already installed to make script idempotent
      "if ! command -v docker >/dev/null 2>&1; then",
      "  echo 'Docker not found. Installing...'",
      "  curl -fsSL https://get.docker.com -o get-docker.sh",
      "  sh get-docker.sh",
      "  rm get-docker.sh",
      "  usermod -aG docker ${var.ssh_user}",
      "  systemctl enable docker",
      "  systemctl start docker",
      "else",
      "  echo 'Docker is already installed. Skipping installation.'",
      "fi",
      "docker --version",
      "docker compose version"
    ]
  }
}
