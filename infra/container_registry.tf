# v2: container images for the back-end (built from backend/). Each build is
# tagged with its git commit hash, and those tags can't be overwritten, so a
# hash always means one build. The one exception is "latest", which CI moves
# to each new build; the ECS task definition runs "latest".
resource "aws_ecr_repository" "registry" {
  name                 = "blue-report-registry"
  image_tag_mutability = "IMMUTABLE_WITH_EXCLUSION"

  image_tag_mutability_exclusion_filter {
    filter      = "latest"
    filter_type = "WILDCARD"
  }

  # Free check for known vulnerabilities; results show in the console.
  image_scanning_configuration {
    scan_on_push = true
  }
}

# v2: keep the last 10 images, which is plenty to roll back to.
resource "aws_ecr_lifecycle_policy" "registry" {
  repository = aws_ecr_repository.registry.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep the last 10 images"
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = 10
        }
        action = {
          type = "expire"
        }
      }
    ]
  })
}
