resource "null_resource" "this" {}

data "archive_file" "app_scheduler_function_zip" {
  type        = "zip"
  output_path = "./build/app-scheduler/bootstrap"
  source {
    content  = "dummy"
    filename = "bootstrap"
  }
  depends_on = [
    null_resource.this
  ]
}

# terraform 経由でリリースする場合は、以下をコメントアウトする
# data "archive_file" "app_scheduler_function_zip" {
#   type        = "zip"
#   source_file = "./build/app-scheduler/bootstrap"
#   output_path = "./build/app-scheduler/bootstrap"
# }

locals {
  function_name = "aws-scheduler"
}

resource "aws_cloudwatch_log_group" "this" {
  name              = "/aws/lambda/${var.function_name}"
  retention_in_days = 1
}

resource "aws_lambda_function" "app_scheduler_function" {
  function_name = var.function_name

  handler          = "main"
  filename         = data.archive_file.app_scheduler_function_zip.output_path
  runtime          = "provided.al2"
  architectures    = ["arm64"]
  role             = aws_iam_role.crypto_trading_lambda_iam_role.arn
  source_code_hash = data.archive_file.app_scheduler_function_zip.output_base64sha256

  memory_size = 128
  timeout     = 600

  lifecycle {
    ignore_changes = [source_code_hash]
  }

  environment {
    variables = {
      "SLACK_URL" = var.slack_url
    }
  }

  depends_on = [aws_cloudwatch_log_group.this]
}
