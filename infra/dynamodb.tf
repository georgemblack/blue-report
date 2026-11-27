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
  hash_key     = "updatedAtMonth"
  range_key    = "updatedAt"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "updatedAtMonth"
    type = "S"
  }

  attribute {
    name = "updatedAt"
    type = "S"
  }
}

resource "aws_dynamodb_table" "url_translations_test" {
  name         = "blue-report-url-translations-test"
  hash_key     = "updatedAtMonth"
  range_key    = "updatedAt"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "updatedAtMonth"
    type = "S"
  }

  attribute {
    name = "updatedAt"
    type = "S"
  }
}

resource "aws_dynamodb_table" "feed" {
  name         = "blue-report-feed"
  hash_key     = "urlHash"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "urlHash"
    type = "S"
  }
}

resource "aws_dynamodb_table" "feed_test" {
  name         = "blue-report-feed-test"
  hash_key     = "urlHash"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "urlHash"
    type = "S"
  }
}

# v2: what the engine's redirect resolver found for each popular link: where
# it redirects to, or that it doesn't. Items expire after 10 days.
resource "aws_dynamodb_table" "redirects" {
  name         = "blue-report-redirects"
  hash_key     = "urlHash"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "urlHash"
    type = "S"
  }

  # DynamoDB deletes items once the time in expiresAt has passed.
  ttl {
    attribute_name = "expiresAt"
    enabled        = true
  }
}
