output "endpoint" {
  description = "The connection endpoint of the PostgreSQL instance"
  value       = aws_db_instance.postgres.endpoint
}

output "address" {
  description = "The DNS hostname of the PostgreSQL instance"
  value       = aws_db_instance.postgres.address
}

output "port" {
  description = "The database port"
  value       = aws_db_instance.postgres.port
}

output "database_name" {
  description = "The database name"
  value       = aws_db_instance.postgres.db_name
}

output "secret_arn" {
  description = "ARN of the Secrets Manager secret storing database credentials"
  value       = aws_secretsmanager_secret.db_secret.arn
}
