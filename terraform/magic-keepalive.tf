resource "aws_iam_role" "keepalive_exec" {
  name = "${var.project_name}-${var.env}-keepalive-exec-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "keepalive_basic" {
  role       = aws_iam_role.keepalive_exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy" "keepalive_ssm" {
  name = "${var.project_name}-${var.env}-keepalive-ssm"
  role = aws_iam_role.keepalive_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["ssm:GetParameter", "ssm:PutParameter"]
        Resource = local.magic_link_ssm_arn
      },
      {
        Effect   = "Allow"
        Action   = ["kms:Decrypt", "kms:Encrypt", "kms:GenerateDataKey"]
        Resource = "*"
        Condition = {
          StringEquals = { "kms:ViaService" = "ssm.${data.aws_region.current.name}.amazonaws.com" }
        }
      }
    ]
  })
}

resource "aws_lambda_function" "magic_keepalive" {
  function_name    = "${var.project_name}-magic-keepalive-${var.env}"
  runtime          = "provided.al2023"
  handler          = "bootstrap"
  role             = aws_iam_role.keepalive_exec.arn
  filename         = local.keepalive_zip
  source_code_hash = filebase64sha256(local.keepalive_zip)
  timeout          = 30
  memory_size      = 128
  architectures    = ["arm64"]
  description      = "Foam App Review magic-link keepalive ${var.env}"

  environment {
    variables = {
      REVIEWER_ACCOUNT_REFRESH_ENABLED = var.reviewer_account_refresh_enabled ? "true" : "false"
      MAGIC_LINK_SSM_PARAM             = local.magic_link_ssm_param
      TWITCH_CLIENT_ID                 = var.twitch_client_id
      TWITCH_CLIENT_SECRET             = var.twitch_client_secret
      REFRESH_DSN                      = var.proxy_dsn
      SENTRY_ENVIRONMENT               = var.env
      SENTRY_RELEASE                   = var.git_sha
    }
  }

  tags = merge(var.tags, {
    Environment = var.env
  })
}

resource "aws_cloudwatch_log_group" "magic_keepalive_logs" {
  name              = "/aws/lambda/${aws_lambda_function.magic_keepalive.function_name}"
  retention_in_days = 1
  log_group_class   = "STANDARD"

  tags = {
    Environment = var.env
    Service     = "foam"
    s3export    = "true"
  }
}

resource "aws_cloudwatch_event_rule" "magic_keepalive" {
  name                = "${var.project_name}-magic-keepalive-${var.env}"
  description         = "Refresh the App Review magic-link token"
  schedule_expression = "rate(3 hours)"
  state               = var.reviewer_account_refresh_enabled ? "ENABLED" : "DISABLED"
}

resource "aws_cloudwatch_event_target" "magic_keepalive" {
  rule      = aws_cloudwatch_event_rule.magic_keepalive.name
  target_id = "magic-keepalive"
  arn       = aws_lambda_function.magic_keepalive.arn
}

resource "aws_lambda_permission" "magic_keepalive_events" {
  statement_id  = "AllowExecutionFromEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.magic_keepalive.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.magic_keepalive.arn
}
