terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.23.0"
    }
  }
}

variable "namespace" {
  description = "Kubernetes namespace for the network policies"
  type        = string
  default     = "sentinelmesh"
}

# Network policy — allow internal traffic between SentinelMesh services
resource "kubernetes_network_policy" "allow_internal" {
  metadata {
    name      = "allow-internal-traffic"
    namespace = var.namespace
  }

  spec {
    pod_selector {}
    policy_types = ["Ingress"]

    ingress {
      from {
        namespace_selector {
          match_labels = {
            "app.kubernetes.io/part-of" = "sentinelmesh"
          }
        }
      }
    }
  }
}

# Network policy — allow Prometheus scraping from monitoring namespace
resource "kubernetes_network_policy" "allow_prometheus" {
  metadata {
    name      = "allow-prometheus-scraping"
    namespace = var.namespace
  }

  spec {
    pod_selector {}
    policy_types = ["Ingress"]

    ingress {
      from {
        namespace_selector {
          match_labels = {
            name = "monitoring"
          }
        }
      }
      ports {
        port     = "8080"
        protocol = "TCP"
      }
      ports {
        port     = "8081"
        protocol = "TCP"
      }
      ports {
        port     = "8082"
        protocol = "TCP"
      }
      ports {
        port     = "8083"
        protocol = "TCP"
      }
    }
  }
}
