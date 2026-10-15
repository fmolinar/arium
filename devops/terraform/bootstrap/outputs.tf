output "state_bucket" {
  description = "Set as the TF_STATE_BUCKET repository variable."
  value       = aws_s3_bucket.state.bucket
}

output "archive_bucket" {
  value = aws_s3_bucket.archive.bucket
}

output "permissions_boundary_arn" {
  value = aws_iam_policy.boundary.arn
}
