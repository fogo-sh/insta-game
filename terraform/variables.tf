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