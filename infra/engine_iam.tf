# v2: this AWS account's ID, used to limit who can use the engine's roles.
data "aws_caller_identity" "current" {}

# v2: both engine roles can only be taken on by ECS tasks in this account.
# The account check stops another account's ECS from using them.
locals {
  engine_assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect    = "Allow"
        Principal = { Service = "ecs-tasks.amazonaws.com" }
        Action    = "sts:AssumeRole"
        Condition = {
          StringEquals = { "aws:SourceAccount" = data.aws_caller_identity.current.account_id }
        }
      }
    ]
  })
}

# v2: execution role, used by ECS itself to start the engine's container:
# pull its image, write its logs and read its secrets.
resource "aws_iam_role" "engine_execution" {
  name               = "blue-report-engine-execution"
  assume_role_policy = local.engine_assume_role_policy
}

# v2: what the execution role may do, limited to the engine's own registry
# and log group.
resource "aws_iam_role_policy" "engine_execution" {
  name = "blue-report-engine-execution"
  role = aws_iam_role.engine_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        # Signing in to ECR can't be limited to one repository.
        Effect   = "Allow"
        Action   = "ecr:GetAuthorizationToken"
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage",
        ]
        Resource = aws_ecr_repository.registry.arn
      },
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents",
        ]
        Resource = "${aws_cloudwatch_log_group.engine.arn}:*"
      },
      {
        # Secrets passed to the container as environment variables. They
        # use AWS's default key, so no KMS permission is needed.
        Effect = "Allow"
        Action = "ssm:GetParameters"
        Resource = [
          aws_ssm_parameter.r2_access_key_id.arn,
          aws_ssm_parameter.r2_secret_access_key.arn,
          aws_ssm_parameter.cloudflare_account_id.arn,
          aws_ssm_parameter.cloudflare_api_token.arn,
        ]
      },
    ]
  })
}

# v2: task role, used by the engine program itself while running.
resource "aws_iam_role" "engine" {
  name               = "blue-report-engine"
  assume_role_policy = local.engine_assume_role_policy
}

# v2: what the engine may do: read, write and list event files, read and
# save link titles in v1's metadata table, read and save redirects, and read
# and add feed entries.
resource "aws_iam_role_policy" "engine" {
  name = "blue-report-engine"
  role = aws_iam_role.engine.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
        ]
        Resource = "${aws_s3_bucket.events.arn}/*"
      },
      {
        Effect   = "Allow"
        Action   = "s3:ListBucket"
        Resource = aws_s3_bucket.events.arn
      },
      # Link titles, shared with v1.
      {
        Effect = "Allow"
        Action = [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
        ]
        Resource = aws_dynamodb_table.url_metadata.arn
      },
      {
        Effect = "Allow"
        Action = [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
        ]
        Resource = aws_dynamodb_table.redirects.arn
      },
      {
        Effect = "Allow"
        Action = [
          "dynamodb:Scan",
          "dynamodb:PutItem",
        ]
        Resource = aws_dynamodb_table.feed_entries.arn
      },
    ]
  })
}
