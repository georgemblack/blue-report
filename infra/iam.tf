# IAM role and policy for Blue Report Aggregation ECS services.
# These services require S3 access to read/publish data, as well as access to AWS Secrets Manager.
resource "aws_iam_role" "service" {
  name = "blue-report-service"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect    = "Allow",
        Principal = { Service = "ecs-tasks.amazonaws.com" },
        Action    = "sts:AssumeRole"
      }
    ]
  })
}

resource "aws_iam_policy" "service" {
  name        = "blue-report-service"
  description = "Policy for Blue Report ECS services"

  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect = "Allow",
        Action = [
          "s3:PutObject",
          "s3:PutObjectAcl",
          "s3:GetObject",
          "s3:ListBucket"
        ],
        Resource = ["${aws_s3_bucket.site.arn}/*", "${aws_s3_bucket.site.arn}"]
      },
      {
        Effect = "Allow",
        Action = [
          "dynamodb:PutItem",
          "dynamodb:GetItem",
          "dynamodb:UpdateItem",
          "dynamodb:DeleteItem"
        ],
        Resource = [aws_dynamodb_table.url_metadata.arn]
      },
      {
        Effect = "Allow",
        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:DescribeSecret"
        ],
        Resource = ["arn:aws:secretsmanager:us-west-2:242201310196:secret:blue-report/cloudflare-deploy-hook-url-GVHZlh"]
      }
    ]
  })
}


resource "aws_iam_role_policy_attachment" "service" {
  role       = aws_iam_role.service.name
  policy_arn = aws_iam_policy.service.arn
}

# IAM execution role and policy for Blue Report ECS services.
# The only permissions required are access to CloudWatch Logs and ECR, to deploy services.
resource "aws_iam_role" "execution" {
  name = "blue-report-service-execution"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect    = "Allow",
        Principal = { Service = "ecs-tasks.amazonaws.com" },
        Action    = "sts:AssumeRole"
      }
    ]
  })
}

resource "aws_iam_policy" "execution" {
  name        = "blue-report-service-execution"
  description = "Policy for Blue Report ECS services"

  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect = "Allow",
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ],
        Resource = "*"
      },
      {
        Effect = "Allow",
        Action = [
          "ecr:GetAuthorizationToken",
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage"
        ],
        Resource = [
          "*"
        ]
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "execution" {
  role       = aws_iam_role.execution.name
  policy_arn = aws_iam_policy.execution.arn
}

# IAM role and policy for the EventBridge scheduler.
# Requries permissions to execute the given ECS task.
resource "aws_iam_role" "scheduler" {
  name = "blue-report-scheduler"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect    = "Allow",
        Principal = { Service = "scheduler.amazonaws.com" },
        Action    = "sts:AssumeRole"
      }
    ]
  })
}

data "aws_iam_policy" "scheduler" {
  name = "AmazonEC2ContainerServiceEventsRole"
}

resource "aws_iam_role_policy_attachment" "scheduler" {
  role       = aws_iam_role.scheduler.name
  policy_arn = data.aws_iam_policy.scheduler.arn
}
