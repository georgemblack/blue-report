resource "aws_acm_certificate" "assets" {
  provider          = aws.acm_certificate
  domain_name       = "assets.theblue.report"
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_acm_certificate_validation" "assets" {
  provider        = aws.acm_certificate
  certificate_arn = aws_acm_certificate.assets.arn
}

resource "aws_cloudfront_origin_access_control" "assets" {
  name                              = "blue-report-assets"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_distribution" "assets" {
  aliases = ["assets.theblue.report"]

  viewer_certificate {
    acm_certificate_arn      = aws_acm_certificate.assets.arn
    ssl_support_method       = "sni-only"
    minimum_protocol_version = "TLSv1.2_2021"
  }

  origin {
    domain_name              = aws_s3_bucket.site.bucket_regional_domain_name
    origin_access_control_id = aws_cloudfront_origin_access_control.assets.id
    origin_id                = "s3-origin"
  }

  enabled         = true
  is_ipv6_enabled = true

  default_cache_behavior {
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "s3-origin"
    cache_policy_id        = "658327ea-f89d-4fab-a63d-7e88639e58f6" // Managed-CachingOptimized
    viewer_protocol_policy = "redirect-to-https"
    compress               = true
  }

  price_class  = "PriceClass_100"
  http_version = "http2and3"

  restrictions {
    geo_restriction {
      locations        = []
      restriction_type = "none"
    }
  }

  depends_on = [aws_acm_certificate_validation.assets]
}
