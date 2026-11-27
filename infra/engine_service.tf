# v2: how to run the engine's container. It always runs the "latest" image;
# CI pushes a new "latest" on every push to master and then redeploys the
# service, so Terraform doesn't change when code does.
resource "aws_ecs_task_definition" "engine" {
  family                   = "blue-report-engine"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 512  # 0.5 vCPU, to start; adjust after measuring
  memory                   = 2048 # MB
  task_role_arn            = aws_iam_role.engine.arn
  execution_role_arn       = aws_iam_role.engine_execution.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }

  container_definitions = jsonencode([
    {
      name      = "engine"
      image     = "${aws_ecr_repository.registry.repository_url}:latest"
      essential = true

      # Time to save buffered events after SIGTERM before being killed.
      stopTimeout = 120

      environment = [
        { name = "EVENTS_BUCKET", value = aws_s3_bucket.events.bucket },
        # v1's R2 bucket, shared for now. Everything goes under "engine/",
        # so v1's files are never overwritten.
        { name = "PUBLISH_BUCKET", value = "blue-report" },
        { name = "PUBLISH_PREFIX", value = "engine/" },
        # v1's table of link titles, shared so neither looks a page up twice.
        { name = "METADATA_TABLE", value = aws_dynamodb_table.url_metadata.name },
        # Where the redirect resolver saves what it finds.
        { name = "REDIRECTS_TABLE", value = aws_dynamodb_table.redirects.name },
        # Make Go's garbage collector work harder as memory nears the
        # task's limit, instead of the task being killed (~85% of 2 GB).
        { name = "GOMEMLIMIT", value = "1740MiB" },
      ]

      secrets = [
        { name = "R2_ACCOUNT_ID", valueFrom = aws_ssm_parameter.cloudflare_account_id.arn },
        { name = "R2_ACCESS_KEY_ID", valueFrom = aws_ssm_parameter.r2_access_key_id.arn },
        { name = "R2_SECRET_ACCESS_KEY", valueFrom = aws_ssm_parameter.r2_secret_access_key.arn },
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-region"        = "us-west-2"
          "awslogs-group"         = aws_cloudwatch_log_group.engine.name
          "awslogs-stream-prefix" = "engine"
        }
      }
    },
  ])
}

# v2: keeps one copy of the engine running, in v1's cluster and subnets.
resource "aws_ecs_service" "engine" {
  name            = "engine"
  cluster         = aws_ecs_cluster.blue_report.id
  task_definition = aws_ecs_task_definition.engine.arn

  # Exactly one copy (see the deployment settings below).
  desired_count = 1

  # On-demand Fargate during the parallel run; Spot can come later.
  capacity_provider_strategy {
    capacity_provider = "FARGATE"
    weight            = 1
  }

  # Never run two copies at once (they'd both read Jetstream and write
  # events): stop the old task before starting the new one.
  deployment_minimum_healthy_percent = 0
  deployment_maximum_percent         = 100

  # If a new deployment's task keeps failing to start, go back to the last
  # one that worked.
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  network_configuration {
    subnets = [
      aws_subnet.blue_report_subnet_2a.id,
      aws_subnet.blue_report_subnet_2b.id,
      aws_subnet.blue_report_subnet_2c.id,
    ]
    security_groups  = [aws_security_group.sg.id]
    assign_public_ip = true
  }

  # Copy the service's tags (Project) to its tasks, for cost tracking.
  propagate_tags = "SERVICE"
}
