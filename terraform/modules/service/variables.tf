variable "name" { type = string }
variable "namespace" { type = string }
variable "chart_path" { type = string }
variable "values_file" { type = string }

variable "profile" {
  type    = string
  default = "dev"
  validation {
    condition     = contains(["dev", "prod-like"], var.profile)
    error_message = "profile must be dev or prod-like."
  }
}

variable "image_tag" {
  type    = string
  default = "dev"
}

variable "alert_enabled" {
  type    = bool
  default = false
}
