# v2: logs from the engine's container. Kept for a week; the Parquet event
# files, not the logs, are the record of what happened. The Standard class
# (rather than Infrequent Access) allows metric filters, for alerting on log
# lines later.
resource "aws_cloudwatch_log_group" "engine" {
  name              = "blue-report-engine"
  retention_in_days = 7
  log_group_class   = "STANDARD"
}
