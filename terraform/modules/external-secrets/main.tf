terraform {
  required_providers {
    helm       = { source = "hashicorp/helm", version = "~> 3.0" }
    kubernetes = { source = "hashicorp/kubernetes", version = "~> 2.30" }
  }
}

resource "kubernetes_namespace" "external_secrets" {
  metadata {
    name = var.namespace
  }
}

# External Secrets Operator. The ClusterSecretStore is applied separately (`make eso-store`) because its CRD does
# not exist until this release is installed, and Terraform needs the CRD at plan time.
resource "helm_release" "external_secrets" {
  name       = "external-secrets"
  namespace  = kubernetes_namespace.external_secrets.metadata[0].name
  repository = "https://charts.external-secrets.io"
  chart      = "external-secrets"
  version    = var.chart_version
  timeout    = 300
  wait       = true

  values = [yamlencode({
    installCRDs    = true
    resources      = { requests = { cpu = "20m", memory = "64Mi" }, limits = { memory = "192Mi" } }
    webhook        = { resources = { requests = { cpu = "10m", memory = "32Mi" }, limits = { memory = "128Mi" } } }
    certController = { resources = { requests = { cpu = "10m", memory = "64Mi" }, limits = { memory = "128Mi" } } }
  })]
}
