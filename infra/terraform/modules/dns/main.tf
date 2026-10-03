terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.23.0"
    }
  }
}

variable "namespace" {
  description = "Kubernetes namespace"
  type        = string
  default     = "sentinelmesh"
}

variable "domain" {
  description = "Base domain for SentinelMesh"
  type        = string
  default     = "sentinelmesh.local"
}

# CoreDNS customization for local service discovery
resource "kubernetes_config_map" "coredns_custom" {
  metadata {
    name      = "coredns-custom"
    namespace = "kube-system"
  }

  data = {
    "sentinelmesh.server" = <<-EOF
      ${var.domain}:53 {
        errors
        cache 30
        forward . /etc/resolv.conf
      }
    EOF
  }
}
