# v2: bucket for event files written by intake (Parquet, one file every few
# minutes). Layout and columns: docs/event-schema.md.
resource "aws_s3_bucket" "events" {
  bucket = "blue-report-events"
}

# v2: nothing in the events bucket is public.
resource "aws_s3_bucket_public_access_block" "events" {
  bucket = aws_s3_bucket.events.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# v2: keep events forever, in Intelligent-Tiering, which moves files nobody
# has read for a while to cheaper storage (no fee to read them back). The
# startup rebuild only reads the last week, which stays in the normal tier.
resource "aws_s3_bucket_lifecycle_configuration" "events" {
  bucket = aws_s3_bucket.events.id

  rule {
    id     = "intelligent-tiering"
    status = "Enabled"

    filter {}

    transition {
      days          = 0
      storage_class = "INTELLIGENT_TIERING"
    }
  }

  rule {
    id     = "abort-incomplete-uploads"
    status = "Enabled"

    filter {}

    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }
}
