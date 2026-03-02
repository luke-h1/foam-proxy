locals {
  go_dir         = "${path.module}/../go"
  proxy_zip      = "${local.go_dir}/build/proxy.zip"
  authorizer_zip = "${local.go_dir}/build/authorizer.zip"
}

resource "aws_iam_role" "lambda_exec" {
  name = "${var.project_name}-${var.env}-exec-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Sid    = ""
      Principal = {
        Service = "lambda.amazonaws.com"
      }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_policy" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_lambda_function" "lambda" {
  function_name    = "${var.project_name}-lambda-${var.env}"
  runtime          = "provided.al2023"
  handler          = "bootstrap"
  role             = aws_iam_role.lambda_exec.arn
  filename         = local.proxy_zip
  source_code_hash = filebase64sha256(local.proxy_zip)
  timeout          = 10

  description   = "Foam proxy Lambda ${var.env}"
  memory_size   = 256
  architectures = ["arm64"]
  environment {
    variables = {
      DEPLOYED_AT          = timestamp()
      DEPLOYED_BY          = var.deployed_by
      GIT_SHA              = var.git_sha
      TWITCH_CLIENT_ID     = var.twitch_client_id
      TWITCH_CLIENT_SECRET = var.twitch_client_secret
      PROXY_DSN            = var.proxy_dsn
      SENTRY_ENVIRONMENT   = var.env
      SENTRY_RELEASE       = var.git_sha
    }
  }
  tags = merge(var.tags, {
    Environment = var.env
  })
}

resource "aws_cloudwatch_log_group" "lambda_logs" {
  name              = "/aws/lambda/${aws_lambda_function.lambda.function_name}"
  retention_in_days = 1
  log_group_class   = "STANDARD"

  tags = {
    Environment = var.env
    Service     = "foam"
    s3export    = "true"
  }
}
