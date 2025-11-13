resource "aws_iam_role" "crypto_trading_lambda_iam_role" {
  name = "CryptoTradingLambdaIamRole"

  assume_role_policy = <<POLICY
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
POLICY
}

resource "aws_iam_policy" "crypto_trading_lambda_access_policy" {
  name   = "CryptoTradingLambdaAccessPolicy"
  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "application-autoscaling:*",
        "ecs:*",
        "rds:*",
        "ec2:*",
        "cloudwatch:PutMetricAlarm",
        "cloudwatch:DescribeAlarms",
        "cloudwatch:DeleteAlarms",
        "logs:CreateLogStream",
        "logs:PutLogEvents"
      ],
      "Resource": [
        "*"
      ]
    }
  ]
}
EOF
}

resource "aws_iam_policy_attachment" "crypto_trading_lambda_policy_attach" {
  name       = "app-scheduler-iam-attachment-lambda"
  roles      = [aws_iam_role.crypto_trading_lambda_iam_role.name]
  policy_arn = aws_iam_policy.crypto_trading_lambda_access_policy.arn
}

resource "aws_iam_role" "eventbridge_scheduler_role" {
  name               = "EventBridgeSchedulerRole"
  assume_role_policy = data.aws_iam_policy_document.eventbridge_scheduler_assume.json
}

data "aws_iam_policy_document" "eventbridge_scheduler_assume" {
  statement {
    effect = "Allow"

    actions = [
      "sts:AssumeRole",
    ]

    principals {
      type = "Service"
      identifiers = [
        "scheduler.amazonaws.com",
        "lambda.amazonaws.com",
      ]
    }
  }
}

resource "aws_iam_role_policy" "eventbridge_scheduler_policy" {
  name   = "EventBridgeSchedulerPolicy"
  role   = aws_iam_role.eventbridge_scheduler_role.name
  policy = data.aws_iam_policy_document.eventbridge_scheduler_custom.json
}

data "aws_iam_policy_document" "eventbridge_scheduler_custom" {
  statement {
    effect = "Allow"

    actions = [
      "lambda:InvokeFunction",
    ]

    resources = [
      "*",
    ]
  }
}
