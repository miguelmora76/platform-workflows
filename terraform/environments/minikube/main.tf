terraform {
  required_version = ">= 1.6"
  required_providers {
    helm       = { source = "hashicorp/helm", version = "~> 3.0" }
    kubernetes = { source = "hashicorp/kubernetes", version = "~> 2.30" }
    random     = { source = "hashicorp/random", version = "~> 3.6" }
  }
}

# The cluster itself is created outside Terraform (`make cluster`). Terraform manages what runs in it.
provider "kubernetes" {
  config_path    = var.kubeconfig
  config_context = "minikube"
}

provider "helm" {
  kubernetes = {
    config_path    = var.kubeconfig
    config_context = "minikube"
  }
}

locals {
  services = {
    "claims-rag-java"            = "${path.module}/../../../services/claims-rag-java.yaml"
    "claims-rag-python"          = "${path.module}/../../../services/claims-rag-python.yaml"
    "mcp-devplatform-guardrails" = "${path.module}/../../../services/mcp-devplatform-guardrails.yaml"
  }
  chart_path = "${path.module}/../../../charts/service"
}

resource "kubernetes_namespace" "apps" {
  metadata {
    name = "apps"
  }
}

module "monitoring" {
  count = var.monitoring_enabled ? 1 : 0

  source                 = "../../modules/monitoring"
  chart_version          = var.prometheus_stack_chart_version
  grafana_admin_password = var.grafana_admin_password
}

# Token for the MCP server's HTTP mode. Generated per apply; read it back with `make mcp-token`.
resource "random_password" "mcp_token" {
  length  = 32
  special = false
}

resource "kubernetes_secret" "mcp_tokens" {
  metadata {
    name      = "mcp-guardrails-tokens"
    namespace = kubernetes_namespace.apps.metadata[0].name
  }
  data = {
    GUARDRAILS_TOKENS = "local=mgt_${random_password.mcp_token.result}"
  }
}

module "service" {
  for_each = local.services

  source        = "../../modules/service"
  name          = each.key
  namespace     = kubernetes_namespace.apps.metadata[0].name
  chart_path    = local.chart_path
  values_file   = each.value
  profile       = var.profile
  alert_enabled = var.monitoring_enabled

  depends_on = [module.monitoring, kubernetes_secret.mcp_tokens]
}
