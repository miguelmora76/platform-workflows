variable "namespace" {
  type    = string
  default = "monitoring"
}

variable "chart_version" {
  type        = string
  description = "kube-prometheus-stack chart version. Pinned so applies are repeatable."
}

variable "grafana_admin_password" {
  type        = string
  sensitive   = true
  description = "Local-only Grafana admin password."
}
