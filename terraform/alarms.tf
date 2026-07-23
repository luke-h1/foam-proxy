locals {
  notifications_enabled = var.discord_webhook_url != "" || (var.telegram_bot_token != "" && var.telegram_chat_id != "")

  monitored_functions = {
    proxy = {
      function_name = aws_lambda_function.lambda.function_name
      description   = "Unusual invocation rate on the foam-proxy Lambda (${var.env})"
    }
    authorizer = {
      function_name = aws_lambda_function.api_authorizer.function_name
      description   = "Unusual invocation rate on the foam-proxy authorizer Lambda (${var.env})"
    }
    magic_keepalive = {
      function_name = aws_lambda_function.magic_keepalive.function_name
      description   = "Unusual invocation rate on the foam-proxy magic-keepalive Lambda (${var.env})"
    }
  }
}

resource "aws_sns_topic" "lambda_alarms" {
  count = local.notifications_enabled ? 1 : 0
  name  = "${var.project_name}-lambda-alarms-${var.env}"
  tags = merge(var.tags, {
    Environment = var.env
  })
}

resource "aws_iam_role" "alarm_notifier_exec" {
  count = local.notifications_enabled ? 1 : 0
  name  = "${var.project_name}-${var.env}-alarm-notifier-exec-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "alarm_notifier_basic" {
  count      = local.notifications_enabled ? 1 : 0
  role       = aws_iam_role.alarm_notifier_exec[0].name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_lambda_function" "alarm_notifier" {
  count            = local.notifications_enabled ? 1 : 0
  function_name    = "${var.project_name}-alarm-notifier-${var.env}"
  runtime          = "provided.al2023"
  handler          = "bootstrap"
  role             = aws_iam_role.alarm_notifier_exec[0].arn
  filename         = local.notifier_zip
  source_code_hash = filebase64sha256(local.notifier_zip)
  timeout          = 15
  memory_size      = 128
  architectures    = ["arm64"]
  description      = "Foam proxy CloudWatch alarm notifier ${var.env}"

  environment {
    variables = {
      DISCORD_WEBHOOK_URL = var.discord_webhook_url
      TELEGRAM_BOT_TOKEN  = var.telegram_bot_token
      TELEGRAM_CHAT_ID    = var.telegram_chat_id
      SENTRY_ENVIRONMENT  = var.env
      ENVIRONMENT         = var.env
      GIT_SHA             = var.git_sha
    }
  }

  tags = merge(var.tags, {
    Environment = var.env
  })

  lifecycle {
    precondition {
      condition     = (var.telegram_bot_token == "") == (var.telegram_chat_id == "")
      error_message = "telegram_bot_token and telegram_chat_id must both be set or both empty."
    }
  }
}

resource "aws_cloudwatch_log_group" "alarm_notifier_logs" {
  count             = local.notifications_enabled ? 1 : 0
  name              = "/aws/lambda/${aws_lambda_function.alarm_notifier[0].function_name}"
  retention_in_days = 1
  log_group_class   = "STANDARD"

  tags = {
    Environment = var.env
    Service     = "foam"
    s3export    = "true"
  }
}

resource "aws_sns_topic_subscription" "alarm_notifier" {
  count     = local.notifications_enabled ? 1 : 0
  topic_arn = aws_sns_topic.lambda_alarms[0].arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.alarm_notifier[0].arn
}

resource "aws_lambda_permission" "alarm_notifier_sns" {
  count         = local.notifications_enabled ? 1 : 0
  statement_id  = "AllowExecutionFromSNS"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.alarm_notifier[0].function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.lambda_alarms[0].arn
}

resource "aws_cloudwatch_metric_alarm" "invocations_anomaly" {
  for_each = local.notifications_enabled ? local.monitored_functions : {}

  alarm_name          = "${each.value.function_name}-invocations-anomaly"
  alarm_description   = each.value.description
  comparison_operator = "GreaterThanUpperThreshold"
  evaluation_periods  = 2
  threshold_metric_id = "ad1"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.lambda_alarms[0].arn]
  ok_actions          = [aws_sns_topic.lambda_alarms[0].arn]

  metric_query {
    id          = "ad1"
    expression  = "ANOMALY_DETECTION_BAND(m1, 2)"
    label       = "Invocations (expected)"
    return_data = true
  }

  metric_query {
    id          = "m1"
    return_data = true
    metric {
      metric_name = "Invocations"
      namespace   = "AWS/Lambda"
      period      = 150
      stat        = "Sum"
      dimensions = {
        FunctionName = each.value.function_name
      }
    }
  }

  tags = merge(var.tags, {
    Environment = var.env
  })
}
