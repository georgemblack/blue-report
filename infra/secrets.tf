data "aws_secretsmanager_secret_version" "cache_address" {
  secret_id = "blue-report/cache-address"
}
