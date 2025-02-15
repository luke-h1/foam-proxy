data "archive_file" "lambda_archive" {
  type        = "zip"
  source_dir  = "${path.module}/../dist"
  output_path = "${path.module}/../lambda.zip"
}

resource "aws_iam_role" "lambda_exec" {
  name = "foam-${var.env}-exec-role"
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
# }
resource "aws_iam_role_policy_attachment" "lambda_policy" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

data "aws_iam_policy" "aws_xray_write_only_access" {
  arn = "arn:aws:iam::aws:policy/AWSXrayWriteOnlyAccess"
}
resource "aws_iam_role_policy_attachment" "aws_xray_write_only_access" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = data.aws_iam_policy.aws_xray_write_only_access.arn
}

resource "aws_lambda_function" "lambda" {
  function_name    = "foam-lambda-${var.env}"
  runtime          = "nodejs20.x"
  handler          = "index.handler"
  role             = aws_iam_role.lambda_exec.arn
  filename         = "${path.module}/../lambda.zip"
  source_code_hash = data.archive_file.lambda_archive.output_base64sha256
  timeout          = 10
  tracing_config {
    mode = "Active"
  }
  description   = "foam proxy Lambda ${var.env}"
  memory_size   = 128
  architectures = ["arm64"]
  environment {
    variables = {
      # TWITCH_CLIENT_ID     = var.twitch_client_id
      # TWITCH_CLIENT_SECRET = var.twitch_client_secret
      DEPLOYED_AT = timestamp()
      DEPLOYED_BY = var.deployed_by
      GIT_SHA     = var.git_sha
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
