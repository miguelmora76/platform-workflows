terraform {
  required_providers {
    helm = { source = "hashicorp/helm", version = "~> 3.0" }
  }
}

resource "helm_release" "service" {
  name      = var.name
  namespace = var.namespace
  chart     = var.chart_path
  timeout   = 300
  wait      = true

  # Later files win: chart defaults < profile < the service's own values.
  values = [
    file("${var.chart_path}/profiles/${var.profile}.yaml"),
    file(var.values_file),
    yamlencode({ image = { tag = var.image_tag } }),
    yamlencode({ alert = { enabled = var.alert_enabled } }),
  ]
}
