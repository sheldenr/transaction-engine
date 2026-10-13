output "alb_endpoint" {
  description = "Public ALB URL for Ledger Engine REST API"
  value       = "http://${module.compute.alb_dns_name}"
}

output "vpc_id" {
  description = "Production VPC ID"
  value       = module.vpc.vpc_id
}

output "database_isolated_host" {
  description = "PostgreSQL private host endpoint (in isolated subnet, no public IP)"
  value       = module.database.address
}

output "redis_primary_endpoint" {
  description = "Redis private replication group endpoint"
  value       = module.redis.primary_endpoint_address
}
