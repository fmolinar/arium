# The news collector on AWS Lambda: an EventBridge Scheduler schedule invokes
# the function three times a day, and it writes articles straight to MongoDB
# (Atlas). The function runs outside a VPC so it needs no NAT gateway, which
# keeps the stack within the free tier.

locals {
  name           = "${var.project}-collector"
  mongo_uri_path = "/${var.project}/collector/mongo-uri"
}

data "aws_caller_identity" "current" {}

# The connection string lives in SSM rather than in the function's
# environment, where anyone who can read the function config would see it.
resource "aws_ssm_parameter" "mongo_uri" {
  name        = local.mongo_uri_path
  description = "MongoDB connection string for ${local.name}"
  type        = "SecureString"
  value       = var.mongo_uri
}

resource "aws_cloudwatch_log_group" "collector" {
  name              = "/aws/lambda/${local.name}"
  retention_in_days = var.log_retention_days
}

# Function role

data "aws_iam_policy_document" "lambda_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "collector" {
  name               = "${local.name}-lambda"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
}

data "aws_iam_policy_document" "collector" {
  statement {
    sid       = "Logs"
    actions   = ["logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["${aws_cloudwatch_log_group.collector.arn}:*"]
  }

  # The parameter uses the AWS managed aws/ssm key, whose key policy already
  # lets SSM decrypt for principals in this account.
  statement {
    sid       = "ReadMongoURI"
    actions   = ["ssm:GetParameter"]
    resources = [aws_ssm_parameter.mongo_uri.arn]
  }
}

resource "aws_iam_role_policy" "collector" {
  name   = "collector"
  role   = aws_iam_role.collector.id
  policy = data.aws_iam_policy_document.collector.json
}

# Function

data "archive_file" "collector" {
  type        = "zip"
  source_file = var.lambda_binary
  output_path = "${path.module}/build/collector.zip"
}

resource "aws_lambda_function" "collector" {
  function_name = local.name
  description   = "Collects DevOps/SRE news into MongoDB (cmd/collector in Lambda mode)"
  role          = aws_iam_role.collector.arn

  # provided.al2023 runs the zip's "bootstrap" executable; the collector
  # switches to its Lambda handler when AWS_LAMBDA_RUNTIME_API is set.
  runtime          = "provided.al2023"
  handler          = "bootstrap"
  architectures    = ["arm64"]
  filename         = data.archive_file.collector.output_path
  source_code_hash = data.archive_file.collector.output_base64sha256

  memory_size = var.lambda_memory_mb
  timeout     = var.lambda_timeout_seconds

  environment {
    variables = {
      MONGO_URI_PARAMETER      = aws_ssm_parameter.mongo_uri.name
      MONGO_DB                 = var.mongo_db
      COLLECTOR_SINCE          = var.collector_since
      COLLECTOR_RETENTION      = var.collector_retention
      COLLECTOR_SOURCE_TIMEOUT = var.collector_source_timeout
    }
  }

  logging_config {
    log_format = "Text"
    log_group  = aws_cloudwatch_log_group.collector.name
  }

  depends_on = [aws_iam_role_policy.collector]
}

# Runs are idempotent and the next one catches up, so a failed run isn't
# retried (async invocations otherwise retry twice).
resource "aws_lambda_function_event_invoke_config" "collector" {
  function_name          = aws_lambda_function.collector.function_name
  maximum_retry_attempts = 0
}

# Schedule

data "aws_iam_policy_document" "scheduler_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["scheduler.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [data.aws_caller_identity.current.account_id]
    }
  }
}

resource "aws_iam_role" "scheduler" {
  name               = "${local.name}-scheduler"
  assume_role_policy = data.aws_iam_policy_document.scheduler_assume.json
}

data "aws_iam_policy_document" "scheduler" {
  statement {
    actions   = ["lambda:InvokeFunction"]
    resources = [aws_lambda_function.collector.arn]
  }
}

resource "aws_iam_role_policy" "scheduler" {
  name   = "invoke-collector"
  role   = aws_iam_role.scheduler.id
  policy = data.aws_iam_policy_document.scheduler.json
}

resource "aws_scheduler_schedule" "collector" {
  name                         = local.name
  description                  = "Runs ${local.name}"
  schedule_expression          = var.schedule_expression
  schedule_expression_timezone = var.schedule_timezone
  state                        = var.schedule_enabled ? "ENABLED" : "DISABLED"

  flexible_time_window {
    mode = "OFF"
  }

  target {
    arn      = aws_lambda_function.collector.arn
    role_arn = aws_iam_role.scheduler.arn

    retry_policy {
      maximum_retry_attempts = 0
    }
  }
}
