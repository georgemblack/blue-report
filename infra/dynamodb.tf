resource "aws_dynamodb_table" "url_metadata" {
  name         = "blue-report-url-metadata"
  hash_key     = "urlHash"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "urlHash"
    type = "S"
  }
}

resource "aws_dynamodb_table" "url_metadata_test" {
  name         = "blue-report-url-metadata-test"
  hash_key     = "urlHash"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "urlHash"
    type = "S"
  }
}

resource "aws_dynamodb_table" "url_translations" {
  name         = "blue-report-url-translations"
  hash_key     = "urlHash"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "urlHash"
    type = "S"
  }
}

resource "aws_dynamodb_table" "url_translations_test" {
  name         = "blue-report-url-translations-test"
  hash_key     = "urlHash"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "urlHash"
    type = "S"
  }
}
