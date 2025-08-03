data "archive_file" "auth_archive" {
  type        = "zip"
  source_dir  = "${path.module}/../apps/foam-authorizer/dist"
  output_path = "${path.module}/../authorizer.zip"
}

resource "aws_lambda_function" "api_authorizer" {
  filename         = "${path.module}/../authorizer.zip"
  function_name    = "${var.project_name}-api-authorizer-${var.env}"
  role             = aws_iam_role.lambda_exec.arn
  handler          = "index.handler"
  source_code_hash = data.archive_file.auth_archive.output_base64sha256
  runtime          = "nodejs22.x"
  memory_size      = 256
  architectures    = ["arm64"]
  timeout          = 10

  tracing_config {
    mode = "Active"
  }

  environment {
    variables = {
      API_KEY     = var.api_key
      ENVIRONMENT = var.env
    }
  }

  tags = merge(var.tags, {
    ENVIRONMENT = var.env
  })
}

resource "aws_cloudwatch_log_group" "auth_logs" {
  name              = "/aws/lambda/${aws_lambda_function.api_authorizer.function_name}-logs"
  retention_in_days = 1
  log_group_class   = "STANDARD"

  tags = {
    Environment = var.env
    Service     = "foam"
    s3export    = "true"
  }
}

resource "aws_apigatewayv2_authorizer" "api_key" {
  api_id                            = aws_apigatewayv2_api.lambda.id
  authorizer_type                   = "REQUEST"
  authorizer_uri                    = aws_lambda_function.api_authorizer.invoke_arn
  identity_sources                  = ["$request.querystring.x-api-key"]
  name                              = "api-authorizer"
  authorizer_payload_format_version = "1.0"
  authorizer_result_ttl_in_seconds  = 10
}

resource "aws_lambda_permission" "api_gw_authorizer" {
  statement_id  = "AllowExecutionFromAPIGatewayAuthorizer"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.api_authorizer.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.lambda.execution_arn}/authorizers/${aws_apigatewayv2_authorizer.api_key.id}"
}
