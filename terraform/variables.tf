variable "access_key_id" {
  description = "AWS IAM access key ID."
  type        = string
}

variable "secret_access_key" {
  description = "AWS IAM secret access key."
  type        = string
}

variable "region" {
  description = "AWS region to use."
  type        = string
  default     = "ca-central-1"
}

variable "cidr_block" {
  description = "CIDR block to use for this service's VPC."
  type        = string
  default     = "172.16.0.0/16"
}