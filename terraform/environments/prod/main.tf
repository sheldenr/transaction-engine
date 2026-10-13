# ========================================================
# Production Ledger Engine Infrastructure Deployment
# ========================================================

module "vpc" {
  source = "../../modules/vpc"

  vpc_cidr    = var.vpc_cidr
  environment = var.environment
}

module "security" {
  source = "../../modules/security"

  vpc_id      = module.vpc.vpc_id
  environment = var.environment
  app_port    = 8080
}

module "database" {
  source = "../../modules/database"

  environment                = var.environment
  database_subnet_ids        = module.vpc.database_subnet_ids
  database_security_group_id = module.security.database_security_group_id
  db_instance_class          = var.db_instance_class
  multi_az                   = true
}

module "redis" {
  source = "../../modules/redis"

  environment             = var.environment
  subnet_ids              = module.vpc.database_subnet_ids
  redis_security_group_id = module.security.redis_security_group_id
}

module "compute" {
  source = "../../modules/compute"

  environment           = var.environment
  vpc_id                = module.vpc.vpc_id
  public_subnet_ids     = module.vpc.public_subnet_ids
  app_subnet_ids        = module.vpc.app_subnet_ids
  alb_security_group_id = module.security.alb_security_group_id
  app_security_group_id = module.security.app_security_group_id
  container_image       = var.container_image
  app_port              = 8080
  app_count             = var.app_count

  # Secure connection parameters
  database_url   = "postgres://ledger_admin:secret@${module.database.address}:5432/${module.database.database_name}?sslmode=require"
  redis_addr     = "${module.redis.primary_endpoint_address}:${module.redis.port}"
  redis_password = module.redis.auth_token
}
