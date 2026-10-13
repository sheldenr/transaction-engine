# ========================================================
# Redis Module: AWS ElastiCache Replication Group
# At-Rest & In-Transit Encryption, Auth Token, Multi-AZ
# ========================================================

resource "random_password" "redis_auth" {
  length  = 32
  special = false
}

resource "aws_elasticache_subnet_group" "redis" {
  name        = "${var.environment}-redis-subnet-group"
  subnet_ids  = var.subnet_ids
  description = "Subnet group for Redis cluster in private isolated tier"
}

resource "aws_elasticache_replication_group" "redis" {
  replication_group_id       = "${var.environment}-ledger-cache"
  description                = "Redis cluster for distributed locks and idempotency caching"
  engine                     = "redis"
  engine_version             = "7.1"
  node_type                  = var.node_type
  num_cache_clusters         = 2
  parameter_group_name       = "default.redis7"
  port                       = 6379
  subnet_group_name          = aws_elasticache_subnet_group.redis.name
  security_group_ids         = [var.redis_security_group_id]
  automatic_failover_enabled = true
  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token                 = random_password.redis_auth.result

  tags = {
    Name        = "${var.environment}-redis-cache"
    Environment = var.environment
  }
}
