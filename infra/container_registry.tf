# v2: container images for the back-end (built from backend/). Tags can't be
# overwritten, so each tag always means one build (e.g. a git commit hash).
resource "aws_ecr_repository" "registry" {
  name                 = "blue-report-registry"
  image_tag_mutability = "IMMUTABLE"

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
