output "primary_endpoint_address" {
  description = "The primary connection endpoint of the Redis replication group"
  value       = aws_elasticache_replication_group.redis.primary_endpoint_address
}

output "port" {
  description = "The Redis port"
  value       = aws_elasticache_replication_group.redis.port
}

output "auth_token" {
  description = "Redis authentication token"
  value       = random_password.redis_auth.result
  sensitive   = true
}
