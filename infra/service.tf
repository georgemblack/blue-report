locals {
  intake_version      = "1.20.3"
  aggregation_version = "2.0.3"
}

resource "aws_ecr_repository" "blue_report" {
  name                 = "blue-report"
  image_tag_mutability = "MUTABLE"
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

resource "aws_ecs_task_definition" "blue_report_intake" {
  family                   = "blue-report-intake"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 512
  memory                   = 1024
  task_role_arn            = aws_iam_role.service.arn
  execution_role_arn       = aws_iam_role.execution.arn

  container_definitions = jsonencode([
    {
      name      = "intake"
      image     = "242201310196.dkr.ecr.us-west-2.amazonaws.com/blue-report:${local.intake_version}"
      essential = true
      command   = ["/intake"]
      environment = [
        {
          name  = "VALKEY_ADDRESS"
          value = data.aws_secretsmanager_secret_version.cache_address.secret_string
        },
        {
          name  = "VALKEY_TLS_ENABLED"
          value = "true"
        },
        {
          name  = "S3_BUCKET_NAME"
          value = "blue-report"
        },
        {
          name  = "S3_ASSETS_BUCKET_NAME"
          value = "blue-report-assets"
        }
      ]
      cpu    = 512
      memory = 1024
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-region" = "us-west-2"
          "awslogs-group"  = aws_cloudwatch_log_stream.blue_report.name
          "awslogs-stream-prefix" : "intake"
        }
      }
    },
  ])

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }
}

resource "aws_ecs_task_definition" "blue_report_link_aggregation" {
  family                   = "blue-report-link-aggregation"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 1024
  memory                   = 2048
  task_role_arn            = aws_iam_role.service.arn
  execution_role_arn       = aws_iam_role.execution.arn

  container_definitions = jsonencode([
    {
      name      = "link-aggregation"
      image     = "242201310196.dkr.ecr.us-west-2.amazonaws.com/blue-report:${local.aggregation_version}"
      essential = true
      command   = ["/link_aggregation"]
      environment = [
        {
          name  = "VALKEY_ADDRESS"
          value = data.aws_secretsmanager_secret_version.cache_address.secret_string
        },
        {
          name  = "VALKEY_TLS_ENABLED"
          value = "true"
        },
        {
          name  = "S3_BUCKET_NAME"
          value = "blue-report"
        },
        {
          name  = "S3_ASSETS_BUCKET_NAME"
          value = "blue-report-assets"
        },
        {
          name  = "DYNAMO_URL_METADATA_TABLE"
          value = "blue-report-url-metadata"
        }
      ]
      cpu    = 1024
      memory = 2048
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-region" = "us-west-2"
          "awslogs-group"  = aws_cloudwatch_log_stream.blue_report.name
          "awslogs-stream-prefix" : "link-aggregation"
        }
      }
    },
  ])

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }
}

resource "aws_ecs_task_definition" "blue_report_site_aggregation" {
  family                   = "blue-report-site-aggregation"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 1024
  memory                   = 8192
  task_role_arn            = aws_iam_role.service.arn
  execution_role_arn       = aws_iam_role.execution.arn

  container_definitions = jsonencode([
    {
      name      = "site-aggregation"
      image     = "242201310196.dkr.ecr.us-west-2.amazonaws.com/blue-report:${local.aggregation_version}"
      essential = true
      command   = ["/site_aggregation"]
      environment = [
        {
          name  = "VALKEY_ADDRESS"
          value = data.aws_secretsmanager_secret_version.cache_address.secret_string
        },
        {
          name  = "VALKEY_TLS_ENABLED"
          value = "true"
        },
        {
          name  = "S3_BUCKET_NAME"
          value = "blue-report"
        },
        {
          name  = "S3_ASSETS_BUCKET_NAME"
          value = "blue-report-assets"
        },
        {
          name  = "DYNAMO_URL_METADATA_TABLE"
          value = "blue-report-url-metadata"
        }
      ]
      cpu    = 1024
      memory = 8192
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-region" = "us-west-2"
          "awslogs-group"  = aws_cloudwatch_log_stream.blue_report.name
          "awslogs-stream-prefix" : "site-aggregation"
        }
      }
    },
  ])

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }
}

resource "aws_ecs_service" "blue_report_intake" {
  name            = "blue-report-intake"
  launch_type     = "FARGATE"
  desired_count   = 1
  cluster         = aws_ecs_cluster.blue_report.id
  task_definition = aws_ecs_task_definition.blue_report_intake.arn

  network_configuration {
    subnets          = [aws_subnet.blue_report_subnet_2a.id, aws_subnet.blue_report_subnet_2b.id, aws_subnet.blue_report_subnet_2c.id]
    assign_public_ip = true
    security_groups  = [aws_security_group.blue_report.id]
  }
}

resource "aws_scheduler_schedule" "blue_report_link_aggregation" {
  name                = "blue-report-link-aggregation-schedule"
  schedule_expression = "rate(1 hours)"

  flexible_time_window {
    mode                      = "FLEXIBLE"
    maximum_window_in_minutes = 5
  }

  target {
    arn      = aws_ecs_cluster.blue_report.arn
    role_arn = aws_iam_role.scheduler.arn

    retry_policy {
      maximum_retry_attempts = 0
    }

    ecs_parameters {
      task_definition_arn = aws_ecs_task_definition.blue_report_link_aggregation.arn
      launch_type         = "FARGATE"

      network_configuration {
        subnets          = [aws_subnet.blue_report_subnet_2a.id, aws_subnet.blue_report_subnet_2b.id, aws_subnet.blue_report_subnet_2c.id]
        assign_public_ip = true
        security_groups  = [aws_security_group.blue_report.id]
      }
    }
  }
}

resource "aws_scheduler_schedule" "blue_report_site_aggregation" {
  name                = "blue-report-site-aggregation-schedule"
  schedule_expression = "rate(1 days)"

  flexible_time_window {
    mode                      = "FLEXIBLE"
    maximum_window_in_minutes = 5
  }

  target {
    arn      = aws_ecs_cluster.blue_report.arn
    role_arn = aws_iam_role.scheduler.arn

    retry_policy {
      maximum_retry_attempts = 0
    }

    ecs_parameters {
      task_definition_arn = aws_ecs_task_definition.blue_report_site_aggregation.arn
      launch_type         = "FARGATE"

      network_configuration {
        subnets          = [aws_subnet.blue_report_subnet_2a.id, aws_subnet.blue_report_subnet_2b.id, aws_subnet.blue_report_subnet_2c.id]
        assign_public_ip = true
        security_groups  = [aws_security_group.blue_report.id]
      }
    }
  }
}
