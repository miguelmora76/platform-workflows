variable "kubeconfig" {
  type    = string
  default = "~/.kube/config"
}

variable "profile" {
  type    = string
  default = "dev"
}

variable "monitoring_enabled" {
  type    = bool
  default = true
}

variable "prometheus_stack_chart_version" {
  type    = string
  default = "91.9.0"
}

variable "grafana_admin_password" {
  type      = string
  sensitive = true
  default   = "admin-local-only"
}
