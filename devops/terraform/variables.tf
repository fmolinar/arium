variable "aws_region" {
  description = "AWS region. Pick the one closest to the MongoDB Atlas cluster, and the same as the bootstrap stack's."
  type        = string
  default     = "us-west-2"
}

variable "project" {
  description = "Prefix for resource names and the Project tag."
  type        = string
  default     = "arium"
}

variable "mongo_uri" {
  description = "MongoDB connection string the collector writes to (Atlas: mongodb+srv://...). Stored as an SSM SecureString."
  type        = string
  sensitive   = true
}

variable "mongo_db" {
  description = "MongoDB database holding the news collection."
  type        = string
  default     = "arium"
}

variable "schedule_expression" {
  description = "EventBridge Scheduler expression for collector runs. Matches COLLECTOR_SCHEDULE in Compose."
  type        = string
  default     = "cron(0 0,8,16 * * ? *)"
}

variable "schedule_timezone" {
  description = "Time zone for schedule_expression."
  type        = string
  default     = "UTC"
}

variable "schedule_enabled" {
  description = "Run on schedule_expression. Set false to keep the function but only invoke it by hand."
  type        = bool
  default     = true
}

variable "collector_since" {
  description = "Skip articles published longer ago than this (COLLECTOR_SINCE). Must not exceed collector_retention."
  type        = string
  default     = "168h"
}

variable "collector_retention" {
  description = "Delete articles fetched longer ago than this (COLLECTOR_RETENTION)."
  type        = string
  default     = "720h"
}

variable "collector_source_timeout" {
  description = "Per-source fetch timeout (COLLECTOR_SOURCE_TIMEOUT). Longer than Compose's 15s because Algolia (Hacker News) sometimes takes over 10s to answer from AWS."
  type        = string
  default     = "90s"
}

variable "lambda_memory_mb" {
  description = "Function memory; CPU scales with it."
  type        = number
  default     = 256
}

variable "lambda_timeout_seconds" {
  description = "Function timeout. A run fetches every source concurrently (collector_source_timeout each) and then writes to MongoDB (up to 1 minute)."
  type        = number
  default     = 180
}

variable "log_retention_days" {
  description = "CloudWatch Logs retention for the function's log group."
  type        = number
  default     = 14
}

variable "archive_enabled" {
  description = "Copy each run's raw payloads and manifest to the archive bucket the bootstrap stack creates."
  type        = bool
  default     = true
}

variable "lambda_binary" {
  description = "Path to the linux/arm64 collector binary built by build.sh."
  type        = string
  default     = "build/bootstrap"
}

variable "alarm_email" {
  description = "Email address for collector alarms. Empty creates the SNS topic without a subscription. AWS emails a confirmation link that must be clicked before alerts arrive."
  type        = string
  default     = ""
}

variable "run_interval_seconds" {
  description = "Time between scheduled runs; the alarm period, so each period holds one run. Keep in step with schedule_expression."
  type        = number
  default     = 28800
}

variable "source_failure_runs" {
  description = "Consecutive runs in which some source must fail before the source alarm fires."
  type        = number
  default     = 3
}

variable "slack_team_id" {
  description = "Slack workspace ID (T...) authorized in the AWS Chatbot console. Empty skips Slack notifications."
  type        = string
  default     = ""
}

variable "slack_channel_id" {
  description = "Slack channel ID (C...) that receives collector alarms. Invite the AWS app to it first (/invite @Amazon Q)."
  type        = string
  default     = "C0CC325HB5G"
}
