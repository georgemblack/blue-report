resource "aws_vpc" "blue_report" {
  cidr_block                       = "10.0.0.0/16"
  enable_dns_support               = true
  enable_dns_hostnames             = true
  assign_generated_ipv6_cidr_block = true

  tags = {
    "Name" = "blue-report-vpc"
  }
}

resource "aws_subnet" "blue_report_subnet_2a" {
  vpc_id            = aws_vpc.blue_report.id
  cidr_block        = "10.0.1.0/24"
  availability_zone = "us-west-2a"

  tags = {
    Name = "blue-report-subnet-2a"
  }
}

resource "aws_subnet" "blue_report_subnet_2b" {
  vpc_id            = aws_vpc.blue_report.id
  cidr_block        = "10.0.2.0/24"
  availability_zone = "us-west-2b"

  tags = {
    Name = "blue-report-subnet-2b"
  }
}

resource "aws_subnet" "blue_report_subnet_2c" {
  vpc_id            = aws_vpc.blue_report.id
  cidr_block        = "10.0.3.0/24"
  availability_zone = "us-west-2c"

  tags = {
    Name = "blue-report-subnet-2c"
  }
}

resource "aws_internet_gateway" "blue_report" {
  vpc_id = aws_vpc.blue_report.id

  tags = {
    Name = "blue-report-igw"
  }
}

resource "aws_route_table" "blue_report" {
  vpc_id = aws_vpc.blue_report.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.blue_report.id
  }

  tags = {
    Name = "blue-report-rt"
  }
}

resource "aws_main_route_table_association" "blue_report" {
  vpc_id         = aws_vpc.blue_report.id
  route_table_id = aws_route_table.blue_report.id
}

resource "aws_security_group" "blue_report" {
  vpc_id = aws_vpc.blue_report.id

  # Egress rule to allow all outbound traffic
  egress {
    description = "Allow all outbound traffic"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # Ingress rule to allow traffic from itself
  ingress {
    description = "Allow all inbound traffic from members of this security group"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    self        = true
  }
}
