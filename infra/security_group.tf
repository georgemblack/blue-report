# v2: security group for intake, in the VPC shared with v1. Intake only
# makes outbound connections (Jetstream, S3, ECR, logs, R2 and so on), so
# nothing is allowed in.
resource "aws_security_group" "sg" {
  name        = "blue-report-sg"
  description = "Intake: outbound only"
  vpc_id      = aws_vpc.blue_report.id

  tags = {
    Name = "blue-report-sg"
  }
}

# v2: allow all outbound traffic, over IPv4.
resource "aws_vpc_security_group_egress_rule" "sg_all_outbound" {
  security_group_id = aws_security_group.sg.id
  description       = "All outbound traffic"
  ip_protocol       = "-1"
  cidr_ipv4         = "0.0.0.0/0"
}
