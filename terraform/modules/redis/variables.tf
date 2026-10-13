variable "environment" {
  description = "Environment name"
  type        = string
  default     = "prod"
}

variable "subnet_ids" {
  description = "Subnet IDs for ElastiCache Redis replication group"
  type        = list(string)
}

variable "redis_security_group_id" {
  description = "Security group ID allowing ECS App access to Redis"
  type        = string
}

variable "node_type" {
  description = "ElastiCache node instance type"
  type        = string
  default     = "cache.t4g.micro"
}
