terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.23.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.11.0"
    }
  }
}

provider "kubernetes" {
  config_path    = var.config_path
  config_context = "kind-${var.cluster_name}"
}

provider "helm" {
  kubernetes {
    config_path    = var.config_path
    config_context = "kind-${var.cluster_name}"
  }
}

# --- Modules ---

module "kubernetes" {
  source    = "./modules/kubernetes"
  namespace = var.namespace
}

module "database" {
  source = "./modules/database"
}

module "monitoring" {
  source = "./modules/monitoring"
}

module "secrets" {
  source    = "./modules/secrets"
  namespace = var.namespace
  depends_on = [module.kubernetes]
}

module "network" {
  source    = "./modules/network"
  namespace = var.namespace
  depends_on = [module.kubernetes]
}

module "dns" {
  source    = "./modules/dns"
  namespace = var.namespace
}

# ArgoCD Installation via Helm
resource "helm_release" "argocd" {
  name             = "argocd"
  repository       = "https://argoproj.github.io/argo-helm"
  chart            = "argo-cd"
  namespace        = "argocd"
  create_namespace = true
  version          = "5.46.7"

  set {
    name  = "server.service.type"
    value = "NodePort"
  }
}

# Nginx Ingress Controller
resource "helm_release" "nginx_ingress" {
  name             = "ingress-nginx"
  repository       = "https://kubernetes.github.io/ingress-nginx"
  chart            = "ingress-nginx"
  namespace        = "ingress-nginx"
  create_namespace = true

  set {
    name  = "controller.service.type"
    value = "NodePort"
  }
}
