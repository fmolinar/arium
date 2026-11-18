variable "aws_region" {
  description = "AWS region, the same as the collector stack's."
  type        = string
  default     = "us-west-2"
}

variable "project" {
  description = "Prefix for resource names. The collector stack derives its names from the same prefix."
  type        = string
  default     = "arium"
}

variable "github_actions_role_name" {
  description = "Existing IAM role that GitHub Actions assumes through OIDC. This stack only attaches its permissions; the role and its trust policy are managed outside Terraform."
  type        = string
  default     = "github-arium"
}

variable "archive_retention_days" {
  description = "Delete archived raw payloads and run manifests after this many days. Matches the collector's 30-day retention by default."
  type        = number
  default     = 30
}

variable "state_version_retention_days" {
  description = "Keep previous Terraform state versions this many days, for recovery."
  type        = number
  default     = 90
}
