data "archive_file" "lambda_archive" {
  type        = "zip"
  source_dir  = "${path.module}/../apps/foam-proxy/dist"
  output_path = "${path.module}/../lambda.zip"
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
      }
    ]
  })
}


resource "aws_iam_role_policy_attachment" "lambda_policy" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# data "aws_iam_policy" "aws_xray_write_only_access" {
#   arn = "arn:aws:iam::aws:policy/AWSXrayWriteOnlyAccess"
# }
# resource "aws_iam_role_policy_attachment" "aws_xray_write_only_access" {
#   role       = aws_iam_role.lambda_exec.name
#   policy_arn = data.aws_iam_policy.aws_xray_write_only_access.arn
# }




resource "aws_lambda_function" "lambda" {
  function_name    = "${var.project_name}-lambda-${var.env}"
  runtime          = "nodejs22.x"
  handler          = "index.handler"
  role             = aws_iam_role.lambda_exec.arn
  filename         = "${path.module}/../lambda.zip"
  source_code_hash = data.archive_file.lambda_archive.output_base64sha256
  timeout          = 10

  # tracing_config {
  #   mode = "Active"
  # }
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
      SENTRY_DSN           = var.sentry_dsn
      SENTRY_ENVIRONMENT   = var.env
      SENTRY_RELEASE       = var.git_sha
      NODE_OPTIONS         = "NODE_OPTIONS='--import ./src/instrument.js'"
    }
  }
  tags = merge(var.tags, {
    Environment = var.env,
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
