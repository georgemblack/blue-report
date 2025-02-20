resource "aws_sqs_queue" "blue_report" {
  name                      = "blue-report-normalization"
  message_retention_seconds = 86400
}

resource "aws_sqs_queue" "blue_report_test" {
  name                      = "blue-report-normalization-test"
  message_retention_seconds = 86400
}
