variable "environment" {
  description = "Environment name"
  type        = string
  default     = "prod"
}

variable "database_subnet_ids" {
  description = "IDs of the private isolated database subnets"
  type        = list(string)
}

variable "database_security_group_id" {
  description = "Security group ID allowing only ECS App access"
  type        = string
}

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t4g.medium"
}

variable "db_name" {
  description = "PostgreSQL database name"
  type        = string
  default     = "ledger_prod"
}

variable "db_username" {
  description = "Master database username"
  type        = string
  default     = "ledger_admin"
}

variable "multi_az" {
  description = "Enable Multi-AZ deployment for high availability"
  type        = bool
  default     = true
}
