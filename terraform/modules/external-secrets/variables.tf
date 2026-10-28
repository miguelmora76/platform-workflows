variable "namespace" {
  type    = string
  default = "external-secrets"
}

variable "chart_version" {
  type        = string
  description = "external-secrets chart version. Pinned so applies are repeatable."
}
