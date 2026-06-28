data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  build_dir      = "${path.module}/.."
  proxy_zip      = "${local.build_dir}/build/proxy.zip"
  authorizer_zip = "${local.build_dir}/build/authorizer.zip"
  keepalive_zip  = "${local.build_dir}/build/magic-keepalive.zip"

  # Canonical store for the App Review magic-link token blob. The proxy reads it at
  # request time and the magic-keepalive Lambda rotates it; seeded from
  # var.magic_link_blob and otherwise left alone (see aws_ssm_parameter below).
  # The whole feature is an explicit on/off switch: enabling requires the blob
  # secret to be seeded too.
  magic_link_enabled   = var.reviewer_account_refresh_enabled
  magic_link_ssm_param = "/${var.project_name}/${var.env}/magic-link-blob"
  magic_link_ssm_arn   = "arn:aws:ssm:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:parameter${local.magic_link_ssm_param}"
}

# SecureString seeded once from var.magic_link_blob (the GitHub secret). value is
# ignored on subsequent applies so a deploy never reverts a token the keepalive
# Lambda has rotated. Re-seeding (e.g. after re-minting) needs a manual SSM update.
resource "aws_ssm_parameter" "magic_link_blob" {
  count = local.magic_link_enabled ? 1 : 0
  name  = local.magic_link_ssm_param
  type  = "SecureString"
  value = var.magic_link_blob
  tags  = var.tags

  lifecycle {
    ignore_changes = [value]
  }
}

# Read-only access to the blob for the shared proxy/authorizer execution role. The
# authorizer never reads it; the grant is scoped to the single parameter.
resource "aws_iam_role_policy" "lambda_magic_link_read" {
  count = local.magic_link_enabled ? 1 : 0
  name  = "${var.project_name}-${var.env}-magic-link-ssm-read"
  role  = aws_iam_role.lambda_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["ssm:GetParameter"]
        Resource = local.magic_link_ssm_arn
      },
      {
        Effect   = "Allow"
        Action   = ["kms:Decrypt"]
        Resource = "*"
        Condition = {
          StringEquals = {
            "kms:ViaService"                      = "ssm.${data.aws_region.current.name}.amazonaws.com"
            "kms:EncryptionContext:PARAMETER_ARN" = local.magic_link_ssm_arn
          }
        }
      }
    ]
  })
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
      MAGIC_LINK_SSM_PARAM = local.magic_link_enabled ? local.magic_link_ssm_param : ""
      MAGIC_LINK_API_KEY   = var.magic_link_api_key
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
