variable "aws_region" {
  description = "AWS deployment region"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment identifier"
  type        = string
  default     = "prod"
}

variable "vpc_cidr" {
  description = "CIDR block for the production VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "container_image" {
  description = "Ledger Engine container image URI"
  type        = string
  default     = "123456789012.dkr.ecr.us-east-1.amazonaws.com/ledger-engine:latest"
}

variable "db_instance_class" {
  description = "RDS instance size"
  type        = string
  default     = "db.t4g.medium"
}

variable "app_count" {
  description = "Number of ECS tasks to maintain"
  type        = number
  default     = 2
}
