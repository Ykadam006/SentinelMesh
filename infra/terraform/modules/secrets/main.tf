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

variable "db_host" {
  description = "Database host"
  type        = string
  default     = "postgresql.database.svc.cluster.local"
}

variable "db_user" {
  description = "Database user"
  type        = string
  default     = "sentinel"
  sensitive   = true
}

variable "db_password" {
  description = "Database password"
  type        = string
  default     = "password"
  sensitive   = true
}

resource "kubernetes_secret" "db_secret" {
  metadata {
    name      = "sentinelmesh-db-secret"
    namespace = var.namespace
  }

  data = {
    DB_HOST     = var.db_host
    DB_USER     = var.db_user
    DB_PASSWORD = var.db_password
    DB_NAME     = "sentinelmesh"
    DB_PORT     = "5432"
  }

  type = "Opaque"
}
