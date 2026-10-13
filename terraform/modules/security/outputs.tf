output "alb_security_group_id" {
  description = "Security group ID for Application Load Balancer"
  value       = aws_security_group.alb.id
}

output "app_security_group_id" {
  description = "Security group ID for Application tasks"
  value       = aws_security_group.app.id
}

output "database_security_group_id" {
  description = "Security group ID for isolated PostgreSQL database"
  value       = aws_security_group.database.id
}

output "redis_security_group_id" {
  description = "Security group ID for ElastiCache Redis"
  value       = aws_security_group.redis.id
}
