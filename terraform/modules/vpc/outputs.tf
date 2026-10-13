output "vpc_id" {
  description = "The ID of the VPC"
  value       = aws_vpc.main.id
}

output "public_subnet_ids" {
  description = "IDs of the public subnets"
  value       = aws_subnet.public[*].id
}

output "app_subnet_ids" {
  description = "IDs of the private app subnets"
  value       = aws_subnet.app[*].id
}

output "database_subnet_ids" {
  description = "IDs of the private isolated database subnets"
  value       = aws_subnet.database[*].id
}
