# ========================================================
# Database Module: Private Isolated RDS PostgreSQL
# Zero Public IP, At-Rest KMS Encryption, Multi-AZ
# ========================================================

# Generate secure random password for database master user
resource "random_password" "db_password" {
  length           = 32
  special          = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

# Store database credentials securely in AWS Secrets Manager
resource "aws_secretsmanager_secret" "db_secret" {
  name                    = "${var.environment}-ledger-db-credentials"
  recovery_window_in_days = 0

  tags = {
    Name        = "${var.environment}-db-credentials"
    Environment = var.environment
  }
}

resource "aws_secretsmanager_secret_version" "db_secret_version" {
  secret_id = aws_secretsmanager_secret.db_secret.id
  secret_string = jsonencode({
    engine   = "postgres"
    host     = aws_db_instance.postgres.address
    port     = aws_db_instance.postgres.port
    username = var.db_username
    password = random_password.db_password.result
    dbname   = var.db_name
  })
}

# DB Subnet Group strictly in Isolated Database Subnets
resource "aws_db_subnet_group" "isolated" {
  name        = "${var.environment}-isolated-db-subnet-group"
  subnet_ids  = var.database_subnet_ids
  description = "Subnet group confined to isolated private subnets with no internet gateway"

  tags = {
    Name        = "${var.environment}-isolated-db-subnet-group"
    Environment = var.environment
  }
}

# RDS Parameter Group for PostgreSQL 16
resource "aws_db_parameter_group" "postgres16" {
  name        = "${var.environment}-pg16-params"
  family      = "postgres16"
  description = "Custom parameter group for fintech ledger consistency"

  parameter {
    name  = "rds.force_ssl"
    value = "1"
  }

  parameter {
    name  = "log_connections"
    value = "1"
  }

  parameter {
    name  = "log_disconnections"
    value = "1"
  }
}

# RDS PostgreSQL Instance
resource "aws_db_instance" "postgres" {
  identifier                  = "${var.environment}-ledger-postgres"
  engine                      = "postgres"
  engine_version              = "16.3"
  instance_class              = var.db_instance_class
  allocated_storage           = 20
  max_allocated_storage       = 100
  storage_type                = "gp3"
  storage_encrypted           = true
  db_name                     = var.db_name
  username                    = var.db_username
  password                    = random_password.db_password.result
  db_subnet_group_name        = aws_db_subnet_group.isolated.name
  vpc_security_group_ids      = [var.database_security_group_id]
  parameter_group_name        = aws_db_parameter_group.postgres16.name
  publicly_accessible         = false # Strictly isolated: NO public IP address
  multi_az                    = var.multi_az
  skip_final_snapshot         = true
  deletion_protection         = false
  backup_retention_period     = 7
  auto_minor_version_upgrade  = true
  allow_major_version_upgrade = false

  tags = {
    Name        = "${var.environment}-ledger-postgres"
    Environment = var.environment
    Compliance  = "PCI-DSS-Ready"
  }
}
