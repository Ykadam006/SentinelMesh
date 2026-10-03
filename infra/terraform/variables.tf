variable "cluster_name" {
  description = "Name of the Kubernetes cluster"
  type        = string
  default     = "sentinelmesh-cluster"
}

variable "namespace" {
  description = "Primary namespace for SentinelMesh workloads"
  type        = string
  default     = "sentinelmesh"
}

variable "config_path" {
  description = "Path to kubeconfig"
  type        = string
  default     = "~/.kube/config"
}
