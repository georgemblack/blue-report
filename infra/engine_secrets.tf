# v2: R2 credentials for publishing (R2 isn't AWS, so it uses its own keys
# rather than IAM). ECS reads these at startup and passes them to the
# engine as environment variables.
#
# Terraform only creates the parameters, with a placeholder value, and then
# ignores the value, so real secrets never appear in these files or in the
# state. Set the real values once with the AWS CLI. While v2 shares v1's R2
# bucket, v1's keys can be copied from Secrets Manager without printing them:
#
#   aws ssm put-parameter --profile george --region us-west-2 --overwrite \
#     --type SecureString --name /blue-report-engine/r2-access-key-id \
#     --value "$(aws secretsmanager get-secret-value --profile george --region us-west-2 \
#       --secret-id blue-report/cloudflare-r2-access-key-id --query SecretString --output text)"
#
# and the same for r2-secret-access-key (from blue-report/cloudflare-r2-secret-access-key).
resource "aws_ssm_parameter" "r2_access_key_id" {
  name  = "/blue-report-engine/r2-access-key-id"
  type  = "SecureString"
  value = "placeholder"

  lifecycle {
    ignore_changes = [value]
  }
}

# v2: see above.
resource "aws_ssm_parameter" "r2_secret_access_key" {
  name  = "/blue-report-engine/r2-secret-access-key"
  type  = "SecureString"
  value = "placeholder"

  lifecycle {
    ignore_changes = [value]
  }
}

# v2: Cloudflare account ID, part of R2's address. It isn't really a secret,
# but this repo is public, so it's kept out of the code. Set it the same way,
# copying from v1's blue-report/cloudflare-account-id.
resource "aws_ssm_parameter" "cloudflare_account_id" {
  name  = "/blue-report-engine/cloudflare-account-id"
  type  = "SecureString"
  value = "placeholder"

  lifecycle {
    ignore_changes = [value]
  }
}
