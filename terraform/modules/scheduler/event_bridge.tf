# 3:00 JST (18:00 UTC前日) に起動
resource "aws_cloudwatch_event_rule" "start_app_event_0300" {
  name        = "StartAppEvent0300"
  description = "start application event at 3:00 JST (18:00 UTC)"

  state = "ENABLED"

  schedule_expression = "cron(0 18 * * ? *)" # 3:00 JST = 18:00 UTC (前日)
}

resource "aws_cloudwatch_event_target" "start_app_event_0300" {
  rule      = aws_cloudwatch_event_rule.start_app_event_0300.name
  target_id = "StartAppEvent0300"
  arn       = aws_lambda_function.app_scheduler_function.arn

  input = jsonencode({
    event_type       = "start",
    application_name = "crypto-trading-app"
  })
}

# 12:30 JST (3:30 UTC) に停止
resource "aws_cloudwatch_event_rule" "stop_app_event_1230" {
  name        = "StopAppEvent1230"
  description = "stop application event at 12:30 JST (3:30 UTC)"

  state = "ENABLED"

  schedule_expression = "cron(30 3 * * ? *)" # 12:30 JST = 3:30 UTC
}

resource "aws_cloudwatch_event_target" "stop_app_event_1230" {
  rule      = aws_cloudwatch_event_rule.stop_app_event_1230.name
  target_id = "StopAppEvent1230"
  arn       = aws_lambda_function.app_scheduler_function.arn

  input = jsonencode({
    event_type       = "stop",
    application_name = "crypto-trading-app"
  })
}

# 16:30 JST (7:30 UTC) に起動
resource "aws_cloudwatch_event_rule" "start_app_event_1630" {
  name        = "StartAppEvent1630"
  description = "start application event at 16:30 JST (7:30 UTC)"

  state = "ENABLED"

  schedule_expression = "cron(30 7 * * ? *)" # 16:30 JST = 7:30 UTC
}

resource "aws_cloudwatch_event_target" "start_app_event_1630" {
  rule      = aws_cloudwatch_event_rule.start_app_event_1630.name
  target_id = "StartAppEvent1630"
  arn       = aws_lambda_function.app_scheduler_function.arn

  input = jsonencode({
    event_type       = "start",
    application_name = "crypto-trading-app"
  })
}

# 23:00 JST (14:00 UTC) に停止
resource "aws_cloudwatch_event_rule" "stop_app_event_2300" {
  name        = "StopAppEvent2300"
  description = "stop application event at 23:00 JST (14:00 UTC)"

  state = "ENABLED"

  schedule_expression = "cron(0 14 * * ? *)" # 23:00 JST = 14:00 UTC
}

resource "aws_cloudwatch_event_target" "stop_app_event_2300" {
  rule      = aws_cloudwatch_event_rule.stop_app_event_2300.name
  target_id = "StopAppEvent2300"
  arn       = aws_lambda_function.app_scheduler_function.arn

  input = jsonencode({
    event_type       = "stop",
    application_name = "crypto-trading-app"
  })
}
