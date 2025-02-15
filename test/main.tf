terraform {
  required_version = ">= 1.0.0"
}

# Configure the AWS provider using the passed-in region
provider "aws" {
  region = var.aws_region
}

variable "aws_region" {
  description = "The AWS region to use"
  type        = string
}

locals {
  test_local = "This is a local value"
}

data "aws_ami" "ubuntu" {
  most_recent = true

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-*-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  # Canonical's official AWS account ID for Ubuntu images
  owners = ["099720109477"]
}

# Output the Ubuntu AMI ID so you can verify the correct image is being used.
output "ubuntu_ami_id" {
  description = "The AMI ID of the latest Ubuntu image"
  value       = data.aws_ami.ubuntu.id
}