resource "aws_ecs_task_definition" "blue_report_link_normalization" {
  family                   = "blue-report-link-normalization"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  task_role_arn            = aws_iam_role.service.arn
  execution_role_arn       = aws_iam_role.execution.arn

  container_definitions = jsonencode([
    {
      name      = "link-normalization"
      image     = "242201310196.dkr.ecr.us-west-2.amazonaws.com/blue-report:${local.aggregation_version}"
      essential = true
      command   = ["/link_normalization"]
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
          name  = "DYNAMO_URL_TRANSLATIONS_TABLE"
          value = aws_dynamodb_table.url_translations.name
        },
        {
          name  = "SQS_NORMALIZATION_QUEUE_NAME"
          value = aws_sqs_queue.blue_report.name
        },
      ]
      cpu    = 256
      memory = 512
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-region" = "us-west-2"
          "awslogs-group"  = aws_cloudwatch_log_stream.blue_report.name
          "awslogs-stream-prefix" : "link-normalization"
        }
      }
    },
  ])

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }
}

resource "aws_ecs_service" "blue_report_link_normalization" {
  name            = "link-normalization"
  desired_count   = 1
  cluster         = aws_ecs_cluster.blue_report.id
  task_definition = aws_ecs_task_definition.blue_report_link_normalization.arn

  network_configuration {
    subnets          = [aws_subnet.blue_report_subnet_2a.id, aws_subnet.blue_report_subnet_2b.id, aws_subnet.blue_report_subnet_2c.id]
    assign_public_ip = true
    security_groups  = [aws_security_group.blue_report.id]
  }

  capacity_provider_strategy {
    capacity_provider = "FARGATE_SPOT"
    weight            = 1
  }
}
