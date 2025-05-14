terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.77.0"
    }
  }

  backend "s3" {
    bucket = "blue-report-terraform"
    key    = "state"
    region = "us-west-2"
  }
}

provider "aws" {
  region = "us-west-2"
}
