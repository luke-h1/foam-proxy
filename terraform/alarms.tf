locals {
  notifications_enabled = nonsensitive(var.discord_webhook_url != "" || (var.telegram_bot_token != "" && var.telegram_chat_id != ""))

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

check "alarm_destinations_configured" {
  assert {
    condition     = local.notifications_enabled
    error_message = "Alarm notifications require discord_webhook_url or both telegram_bot_token and telegram_chat_id."
  }
}

resource "aws_sns_topic" "lambda_alarms" {
  count = local.notifications_enabled ? 1 : 0
  name  = "${var.project_name}-lambda-alarms-${var.env}"
  tags = merge(var.tags, {
    Environment = var.env
  })
}

# allow cloudwatch alarms to publish to the topic
resource "aws_sns_topic_policy" "lambda_alarms" {
  count = local.notifications_enabled ? 1 : 0
  arn   = aws_sns_topic.lambda_alarms[0].arn

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "cloudwatch.amazonaws.com" }
      Action    = "SNS:Publish"
      Resource  = aws_sns_topic.lambda_alarms[0].arn
      Condition = {
        ArnLike      = { "aws:SourceArn" = "arn:aws:cloudwatch:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:alarm:*" }
        StringEquals = { "aws:SourceAccount" = data.aws_caller_identity.current.account_id }
      }
    }]
  })
}

resource "aws_sqs_queue" "alarm_notifier_dlq" {
  count                     = local.notifications_enabled ? 1 : 0
  name                      = "${var.project_name}-alarm-notifier-dlq-${var.env}"
  message_retention_seconds = 1209600 # 14 days
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

resource "aws_iam_role_policy" "alarm_notifier_dlq" {
  count = local.notifications_enabled ? 1 : 0
  name  = "${var.project_name}-${var.env}-alarm-notifier-dlq"
  role  = aws_iam_role.alarm_notifier_exec[0].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["sqs:SendMessage"]
      Resource = [aws_sqs_queue.alarm_notifier_dlq[0].arn]
    }]
  })
}

resource "aws_lambda_function" "alarm_notifier" {
  count            = local.notifications_enabled ? 1 : 0
  function_name    = "${var.project_name}-alarm-notifier-${var.env}"
  runtime          = "provided.al2023"
  handler          = "bootstrap"
  role             = aws_iam_role.alarm_notifier_exec[0].arn
  filename         = local.notifier_zip
  source_code_hash = filebase64sha256(local.notifier_zip)
  timeout          = 30
  memory_size      = 128
  architectures    = ["arm64"]
  description      = "Foam proxy CloudWatch alarm notifier ${var.env}"

  dead_letter_config {
    target_arn = aws_sqs_queue.alarm_notifier_dlq[0].arn
  }

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

  depends_on = [
    aws_iam_role_policy.alarm_notifier_dlq,
    aws_iam_role_policy_attachment.alarm_notifier_basic,
  ]

  lifecycle {
    precondition {
      condition     = (var.telegram_bot_token == "") == (var.telegram_chat_id == "")
      error_message = "telegram_bot_token and telegram_chat_id must both be set or both empty."
    }
  }
}

resource "aws_lambda_function_event_invoke_config" "alarm_notifier" {
  count                        = local.notifications_enabled ? 1 : 0
  function_name                = aws_lambda_function.alarm_notifier[0].function_name
  maximum_retry_attempts       = 2
  maximum_event_age_in_seconds = 3600

  destination_config {
    on_failure {
      destination = aws_sqs_queue.alarm_notifier_dlq[0].arn
    }
  }
}

resource "aws_cloudwatch_log_group" "alarm_notifier_logs" {
  count             = local.notifications_enabled ? 1 : 0
  name              = "/aws/lambda/${aws_lambda_function.alarm_notifier[0].function_name}"
  retention_in_days = 14
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
      period      = 60
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

resource "aws_cloudwatch_metric_alarm" "alarm_notifier_errors" {
  count               = local.notifications_enabled ? 1 : 0
  alarm_name          = "${aws_lambda_function.alarm_notifier[0].function_name}-errors"
  alarm_description   = "Alarm notifier Lambda is failing (${var.env})"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = 60
  statistic           = "Sum"
  threshold           = 1
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.lambda_alarms[0].arn]
  ok_actions          = [aws_sns_topic.lambda_alarms[0].arn]

  dimensions = {
    FunctionName = aws_lambda_function.alarm_notifier[0].function_name
  }

  tags = merge(var.tags, {
    Environment = var.env
  })
}

resource "aws_cloudwatch_metric_alarm" "alarm_notifier_dlq_depth" {
  count               = local.notifications_enabled ? 1 : 0
  alarm_name          = "${aws_sqs_queue.alarm_notifier_dlq[0].name}-depth"
  alarm_description   = "Alarm notifier DLQ has messages (${var.env})"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  metric_name         = "ApproximateNumberOfMessagesVisible"
  namespace           = "AWS/SQS"
  period              = 60
  statistic           = "Maximum"
  threshold           = 1
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.lambda_alarms[0].arn]
  ok_actions          = [aws_sns_topic.lambda_alarms[0].arn]

  dimensions = {
    QueueName = aws_sqs_queue.alarm_notifier_dlq[0].name
  }

  tags = merge(var.tags, {
    Environment = var.env
  })
}
