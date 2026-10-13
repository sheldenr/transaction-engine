variable "environment" {
  description = "Environment name"
  type        = string
  default     = "prod"
}

variable "vpc_id" {
  description = "VPC ID"
  type        = string
}

variable "public_subnet_ids" {
  description = "Public subnet IDs for ALB"
  type        = list(string)
}

variable "app_subnet_ids" {
  description = "Private app subnet IDs for ECS tasks"
  type        = list(string)
}

variable "alb_security_group_id" {
  description = "ALB security group ID"
  type        = string
}

variable "app_security_group_id" {
  description = "ECS app security group ID"
  type        = string
}

variable "container_image" {
  description = "Container image URI"
  type        = string
  default     = "123456789012.dkr.ecr.us-east-1.amazonaws.com/ledger-engine:latest"
}

variable "app_port" {
  description = "Container port"
  type        = number
  default     = 8080
}

variable "app_count" {
  description = "Number of ECS task replicas"
  type        = number
  default     = 2
}

variable "database_url" {
  description = "Database connection URL"
  type        = string
}

variable "redis_addr" {
  description = "Redis address"
  type        = string
}

variable "redis_password" {
  description = "Redis password"
  type        = string
  sensitive   = true
}
