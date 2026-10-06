terraform {
  required_providers {
    helm       = { source = "hashicorp/helm", version = "~> 3.0" }
    kubernetes = { source = "hashicorp/kubernetes", version = "~> 2.30" }
  }
}

resource "kubernetes_namespace" "monitoring" {
  metadata {
    name = var.namespace
  }
}

# Prometheus + Grafana + kube-state-metrics. Sized down for a laptop: no Alertmanager, no node-exporter.
resource "helm_release" "kube_prometheus_stack" {
  name       = "monitoring"
  namespace  = kubernetes_namespace.monitoring.metadata[0].name
  repository = "https://prometheus-community.github.io/helm-charts"
  chart      = "kube-prometheus-stack"
  version    = var.chart_version
  timeout    = 600
  wait       = true

  values = [yamlencode({
    alertmanager = { enabled = false }
    nodeExporter = { enabled = false }
    grafana      = { adminPassword = var.grafana_admin_password }
    prometheus = {
      prometheusSpec = {
        # Pick up PrometheusRule objects from any release, not only this one.
        ruleSelectorNilUsesHelmValues = false
        retention                     = "2d"
        resources                     = { requests = { cpu = "100m", memory = "400Mi" }, limits = { memory = "800Mi" } }
      }
    }
  })]
}
