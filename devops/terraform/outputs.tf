output "function_name" {
  value = aws_lambda_function.collector.function_name
}

output "log_group" {
  value = aws_cloudwatch_log_group.collector.name
}

output "schedule" {
  value = "${aws_scheduler_schedule.collector.schedule_expression} (${aws_scheduler_schedule.collector.schedule_expression_timezone}, ${aws_scheduler_schedule.collector.state})"
}

output "invoke_command" {
  description = "Run the collector once and print its logs."
  value       = "aws lambda invoke --region ${var.aws_region} --function-name ${aws_lambda_function.collector.function_name} --log-type Tail --query LogResult --output text /dev/null | base64 -d"
}
