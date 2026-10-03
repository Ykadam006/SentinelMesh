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

resource "kubernetes_namespace" "sentinelmesh" {
  metadata {
    name = var.namespace
    labels = {
      "app.kubernetes.io/part-of" = "sentinelmesh"
    }
  }
}

resource "kubernetes_config_map" "sentinelmesh_config" {
  metadata {
    name      = "sentinelmesh-config"
    namespace = kubernetes_namespace.sentinelmesh.metadata[0].name
  }

  data = {
    KAFKA_BROKER     = "sentinelmesh-kafka:9092"
    METRICS_API_HOST = "metrics-api:50051"
    INCIDENT_API_HOST = "incident-api:50052"
    PROMETHEUS_URL   = "http://prometheus.monitoring.svc.cluster.local:9090"
    ELASTICSEARCH_URL = "http://elasticsearch.sentinelmesh.svc.cluster.local:9200"
  }
}
