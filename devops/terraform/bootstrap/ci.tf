# Permissions for the GitHub Actions role (.github/workflows/collector.yaml)
# to plan, apply and destroy the collector stack, and nothing else. Resources
# are scoped by the collector's name prefix.
#
# The stack creates IAM roles, and a principal that can create roles and pass
# them to Lambda could otherwise grant itself anything through them. So every
# role it creates must carry the permissions boundary below, which caps them at
# what the collector needs; the CI role can't create a role without it or
# remove it afterwards.

locals {
  region_account   = "${var.aws_region}:${local.account_id}"
  role_arns        = "arn:aws:iam::${local.account_id}:role/${local.collector}-*"
  function_arn     = "arn:aws:lambda:${local.region_account}:function:${local.collector}"
  log_group_arn    = "arn:aws:logs:${local.region_account}:log-group:/aws/lambda/${local.collector}"
  parameter_arns   = "arn:aws:ssm:${local.region_account}:parameter/${var.project}/collector/*"
  alerts_topic_arn = "arn:aws:sns:${local.region_account}:${local.collector}-alerts"
}

# Boundary for the collector's own roles (Lambda execution, scheduler, Chatbot).

data "aws_iam_policy_document" "boundary" {
  statement {
    sid       = "Logs"
    actions   = ["logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["${local.log_group_arn}:*"]
  }

  statement {
    sid       = "ReadMongoURI"
    actions   = ["ssm:GetParameter"]
    resources = [local.parameter_arns]
  }

  statement {
    sid     = "Archive"
    actions = ["s3:PutObject"]
    resources = [
      "${aws_s3_bucket.archive.arn}/raw/*",
      "${aws_s3_bucket.archive.arn}/runs/*",
    ]
  }

  statement {
    sid       = "InvokeCollector"
    actions   = ["lambda:InvokeFunction"]
    resources = [local.function_arn, "${local.function_arn}:*"]
  }
}

resource "aws_iam_policy" "boundary" {
  name        = "${local.collector}-boundary"
  description = "Permissions boundary for roles the collector stack creates"
  policy      = data.aws_iam_policy_document.boundary.json
}

# The CI role's permissions.

data "aws_iam_policy_document" "ci" {
  statement {
    sid       = "StateList"
    actions   = ["s3:ListBucket"]
    resources = [aws_s3_bucket.state.arn]
  }

  # Includes the .tflock lockfile next to the state.
  statement {
    sid       = "StateObjects"
    actions   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
    resources = ["${aws_s3_bucket.state.arn}/collector/*"]
  }

  statement {
    sid       = "CreateBoundedRoles"
    actions   = ["iam:CreateRole", "iam:PutRolePolicy"]
    resources = [local.role_arns]
    condition {
      test     = "StringEquals"
      variable = "iam:PermissionsBoundary"
      values   = [aws_iam_policy.boundary.arn]
    }
  }

  statement {
    sid = "ManageRoles"
    actions = [
      "iam:GetRole",
      "iam:UpdateRole",
      "iam:UpdateAssumeRolePolicy",
      "iam:DeleteRole",
      "iam:TagRole",
      "iam:UntagRole",
      "iam:GetRolePolicy",
      "iam:ListRolePolicies",
      "iam:DeleteRolePolicy",
      "iam:ListAttachedRolePolicies",
      "iam:ListInstanceProfilesForRole",
    ]
    resources = [local.role_arns]
  }

  statement {
    sid       = "PassRolesToCollector"
    actions   = ["iam:PassRole"]
    resources = [local.role_arns]
    condition {
      test     = "StringEquals"
      variable = "iam:PassedToService"
      values   = ["lambda.amazonaws.com", "scheduler.amazonaws.com"]
    }
  }

  statement {
    sid       = "KeepBoundary"
    effect    = "Deny"
    actions   = ["iam:DeleteRolePermissionsBoundary", "iam:PutRolePermissionsBoundary"]
    resources = ["*"]
  }

  statement {
    sid       = "ReadBoundary"
    actions   = ["iam:GetPolicy", "iam:GetPolicyVersion"]
    resources = [aws_iam_policy.boundary.arn]
  }

  # Also lets the workflow invoke the function after an apply.
  statement {
    sid       = "Function"
    actions   = ["lambda:*"]
    resources = [local.function_arn, "${local.function_arn}:*"]
  }

  statement {
    sid       = "Schedule"
    actions   = ["scheduler:*"]
    resources = ["arn:aws:scheduler:${local.region_account}:schedule/default/${local.collector}"]
  }

  statement {
    sid = "MongoURIParameter"
    actions = [
      "ssm:PutParameter",
      "ssm:GetParameter",
      "ssm:GetParameters",
      "ssm:DeleteParameter",
      "ssm:AddTagsToResource",
      "ssm:RemoveTagsFromResource",
      "ssm:ListTagsForResource",
    ]
    resources = [local.parameter_arns]
  }

  statement {
    sid = "LogGroup"
    actions = [
      "logs:CreateLogGroup",
      "logs:DeleteLogGroup",
      "logs:PutRetentionPolicy",
      "logs:DeleteRetentionPolicy",
      "logs:TagResource",
      "logs:UntagResource",
      "logs:ListTagsForResource",
      "logs:TagLogGroup",
      "logs:UntagLogGroup",
      "logs:ListTagsLogGroup",
    ]
    resources = [local.log_group_arn, "${local.log_group_arn}:*"]
  }

  statement {
    sid       = "SourceFailureMetricFilter"
    actions   = ["logs:PutMetricFilter", "logs:DeleteMetricFilter"]
    resources = [local.log_group_arn, "${local.log_group_arn}:*"]
  }

  statement {
    sid = "Alarms"
    actions = [
      "cloudwatch:PutMetricAlarm",
      "cloudwatch:DeleteAlarms",
      "cloudwatch:TagResource",
      "cloudwatch:UntagResource",
      "cloudwatch:ListTagsForResource",
    ]
    resources = ["arn:aws:cloudwatch:${local.region_account}:alarm:${local.collector}-*"]
  }

  statement {
    sid = "AlertsTopic"
    actions = [
      "sns:CreateTopic",
      "sns:DeleteTopic",
      "sns:GetTopicAttributes",
      "sns:SetTopicAttributes",
      "sns:TagResource",
      "sns:UntagResource",
      "sns:ListTagsForResource",
      "sns:Subscribe",
      "sns:Unsubscribe",
      "sns:GetSubscriptionAttributes",
      "sns:SetSubscriptionAttributes",
      "sns:ListSubscriptionsByTopic",
    ]
    resources = [local.alerts_topic_arn, "${local.alerts_topic_arn}:*"]
  }

  statement {
    sid       = "PassRoleToChatbot"
    actions   = ["iam:PassRole"]
    resources = ["arn:aws:iam::${local.account_id}:role/${local.collector}-chatbot"]
    condition {
      test     = "StringEquals"
      variable = "iam:PassedToService"
      values   = ["chatbot.amazonaws.com"]
    }
  }

  # Chatbot has no resource-level permissions for most actions.
  statement {
    sid = "SlackChannel"
    actions = [
      "chatbot:CreateSlackChannelConfiguration",
      "chatbot:UpdateSlackChannelConfiguration",
      "chatbot:DeleteSlackChannelConfiguration",
      "chatbot:DescribeSlackChannelConfigurations",
      "chatbot:TagResource",
      "chatbot:UntagResource",
      "chatbot:ListTagsForResource",
    ]
    resources = ["*"]
  }

  # These list actions don't support resource-level permissions.
  statement {
    sid = "Describe"
    actions = [
      "logs:DescribeLogGroups",
      "logs:DescribeMetricFilters",
      "ssm:DescribeParameters",
      "cloudwatch:DescribeAlarms",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "ci" {
  name   = "${local.collector}-terraform"
  role   = var.github_actions_role_name
  policy = data.aws_iam_policy_document.ci.json
}
