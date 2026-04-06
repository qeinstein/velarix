# Control Plane Infrastructure: AWS Production Cluster
# This provisions the managed cloud environment for Velarix SaaS.

provider "aws" {
  region = var.aws_region
}

variable "aws_region" {
  default = "us-east-1"
}

# 1. High-Performance Postgres (Aurora)
# Stores historical audit logs, explanations, and configurations.
resource "aws_rds_cluster" "velarix_db" {
  cluster_identifier      = "velarix-prod-cluster"
  engine                  = "aurora-postgresql"
  engine_version          = "14.6"
  master_username         = "velarix_admin"
  master_password         = var.db_password
  backup_retention_period = 7
  skip_final_snapshot     = true
}

resource "aws_rds_cluster_instance" "velarix_db_instances" {
  count              = 2
  identifier         = "velarix-prod-instance-${count.index}"
  cluster_identifier = aws_rds_cluster.velarix_db.id
  instance_class     = "db.r6g.large"
  engine             = aws_rds_cluster.velarix_db.engine
  engine_version     = aws_rds_cluster.velarix_db.engine_version
}

# 2. Redis Cluster (ElastiCache)
# Handles distributed rate-limiting and idempotency keys across the fleet.
resource "aws_elasticache_cluster" "velarix_redis" {
  cluster_id           = "velarix-prod-redis"
  engine               = "redis"
  node_type            = "cache.m6g.large"
  num_cache_nodes      = 1
  parameter_group_name = "default.redis6.x"
  port                 = 6379
}

# 3. ECS Fargate Cluster (Go Engine)
# Stateless compute fleet running the core reasoning engine.
resource "aws_ecs_cluster" "velarix_compute" {
  name = "velarix-prod-compute"
}

resource "aws_ecs_task_definition" "velarix_engine" {
  family                   = "velarix-engine-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = 2048
  memory                   = 4096

  container_definitions = jsonencode([{
    name  = "velarix-server"
    image = "velarix/engine:latest"
    portMappings = [{
      containerPort = 8080
      hostPort      = 8080
    }]
    environment = [
      { name = "VELARIX_ENV", value = "prod" },
      { name = "VELARIX_STORE_BACKEND", value = "postgres" },
      { name = "VELARIX_POSTGRES_DSN", value = "postgres://${aws_rds_cluster.velarix_db.master_username}:${var.db_password}@${aws_rds_cluster.velarix_db.endpoint}:5432/velarix?sslmode=require" },
      { name = "VELARIX_REDIS_URL", value = "redis://${aws_elasticache_cluster.velarix_redis.cache_nodes.0.address}:6379" }
    ]
  }])
}
