terraform {
  required_version = ">= 1.0.0"
}

variable "test_variable" {
  description = "A test variable"
  type        = string
  default     = "Hello, Terraform!"
}

variable "aws_region" {
  description = "The AWS region to use"
  type        = string
}

locals {
  test_local = "This is a local value"
}

output "test_output" {
  description = "A test output value"
  value       = "Variable: ${var.test_variable}, Local: ${local.test_local}"
}