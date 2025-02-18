resource "aws_apigatewayv2_api" "lambda" {
  name                         = "foam-gw-${var.env}"
  protocol_type                = "HTTP"
  disable_execute_api_endpoint = true
  cors_configuration {
    allow_headers  = ["*"]
    allow_origins  = ["*"]
    allow_methods  = ["*"]
    expose_headers = ["*"]
  }

  tags = merge(var.tags, {
    Environment = var.env
  })
}

resource "aws_apigatewayv2_domain_name" "domain_name" {
  domain_name = var.env == "live" ? "foam.${var.root_domain}" : "foam-${var.env}.${var.root_domain}"

  domain_name_configuration {
    certificate_arn = aws_acm_certificate.cert.arn
    endpoint_type   = "REGIONAL"
    security_policy = "TLS_1_2"
  }
  tags = merge(var.tags, {
    Environment = var.env
  })
}

resource "aws_apigatewayv2_api_mapping" "lambda" {
  api_id      = aws_apigatewayv2_api.lambda.id
  domain_name = aws_apigatewayv2_domain_name.domain_name.domain_name
  stage       = aws_apigatewayv2_stage.lambda.id
}

resource "aws_apigatewayv2_stage" "lambda" {
  api_id      = aws_apigatewayv2_api.lambda.id
  name        = var.env
  auto_deploy = true
  route_settings {
    route_key              = "$default"
    throttling_burst_limit = 10000
    throttling_rate_limit  = 20000
    logging_level          = "OFF"
  }
  default_route_settings {
    throttling_burst_limit = 10000
    throttling_rate_limit  = 20000
    logging_level          = "OFF"
  }
  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.api_gw.arn
    format = jsonencode({
      requestId               = "$context.requestId"
      sourceIp                = "$context.identity.sourceIp"
      requestTime             = "$context.requestTime"
      protocol                = "$context.protocol"
      httpMethod              = "$context.httpMethod"
      resourcePath            = "$context.resourcePath"
      routeKey                = "$context.routeKey"
      status                  = "$context.status"
      responseLength          = "$context.responseLength"
      integrationErrorMessage = "$context.integrationErrorMessage"
      }
    )
  }
  tags = merge(var.tags, {
    Environment = var.env
  })
}

resource "aws_apigatewayv2_integration" "lambda" {
  api_id             = aws_apigatewayv2_api.lambda.id
  integration_uri    = aws_lambda_function.lambda.invoke_arn
  integration_type   = "AWS_PROXY"
  integration_method = "POST"
}

# ROUTES 
##############################################################################
resource "aws_apigatewayv2_route" "lambda_route_proxy" {
  api_id         = aws_apigatewayv2_api.lambda.id
  target         = "integrations/${aws_apigatewayv2_integration.lambda.id}"
  route_key      = "GET /api/proxy"
  operation_name = "get proxy"
}

resource "aws_apigatewayv2_route" "lambda_route_pending" {
  api_id         = aws_apigatewayv2_api.lambda.id
  target         = "integrations/${aws_apigatewayv2_integration.lambda.id}"
  route_key      = "GET /api/pending"
  operation_name = "get pending"
}

resource "aws_apigatewayv2_route" "lambda_route_healthcheck" {
  api_id         = aws_apigatewayv2_api.lambda.id
  target         = "integrations/${aws_apigatewayv2_integration.lambda.id}"
  route_key      = "GET /api/healthcheck"
  operation_name = "get healthcheck"
}

resource "aws_apigatewayv2_route" "lambda_route_head_healthcheck" {
  api_id         = aws_apigatewayv2_api.lambda.id
  target         = "integrations/${aws_apigatewayv2_integration.lambda.id}"
  route_key      = "HEAD /api/healthcheck"
  operation_name = "head healthcheck"
}

resource "aws_apigatewayv2_route" "lambda_route_version" {
  api_id         = aws_apigatewayv2_api.lambda.id
  target         = "integrations/${aws_apigatewayv2_integration.lambda.id}"
  route_key      = "GET /api/version"
  operation_name = "get version"
}

resource "aws_apigatewayv2_route" "lambda_route_token" {
  api_id         = aws_apigatewayv2_api.lambda.id
  target         = "integrations/${aws_apigatewayv2_integration.lambda.id}"
  route_key      = "GET /api/token"
  operation_name = "get token"
}
##############################################################################


resource "aws_cloudwatch_log_group" "api_gw" {
  name              = "/aws/api_gw/${aws_apigatewayv2_api.lambda.name}"
  retention_in_days = 1
  log_group_class   = "INFREQUENT_ACCESS"

  tags = merge(var.tags, {
    Environment = var.env
  })
}

resource "aws_lambda_permission" "api_gw" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.lambda.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.lambda.execution_arn}/*/*"
}
