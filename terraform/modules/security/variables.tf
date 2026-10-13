variable "vpc_id" {
  description = "The VPC ID"
  type        = string
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "prod"
}

variable "app_port" {
  description = "Application listening port"
  type        = number
  default     = 8080
}
