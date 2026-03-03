resource "aws_ecr_repository" "blue_report" {
  name                 = "blue-report"
  image_tag_mutability = "MUTABLE"
}

resource "aws_ecr_lifecycle_policy" "blue_report" {
  repository = aws_ecr_repository.blue_report.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep last 20 images"
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = 20
        }
        action = {
          type = "expire"
        }
      }
    ]
  })
}

resource "aws_cloudwatch_log_group" "blue_report" {
  name              = "blue-report"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_stream" "blue_report" {
  name           = "blue-report"
  log_group_name = aws_cloudwatch_log_group.blue_report.name
}

resource "aws_ecs_cluster" "blue_report" {
  name = "blue-report"
}

resource "aws_ecs_cluster_capacity_providers" "blue_report" {
  cluster_name = aws_ecs_cluster.blue_report.name
  capacity_providers = [
    "FARGATE",
    "FARGATE_SPOT",
  ]

  default_capacity_provider_strategy {
    capacity_provider = "FARGATE"
  }
}
