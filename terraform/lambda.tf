locals {
  lambda_function_name = "gates"
}

# CloudWatch Logs の既定の暗号化で足りる。ログに資格情報を記録せず、GATES の KMS Key は署名専用で流用できないため、暗号化のためだけに鍵を増やさない
#trivy:ignore:AWS-0017
resource "aws_cloudwatch_log_group" "lambda" {
  name              = "/aws/lambda/${local.lambda_function_name}"
  retention_in_days = var.log_retention_in_days
}

resource "aws_iam_role" "lambda" {
  name                 = "${local.lambda_function_name}-lambda"
  permissions_boundary = aws_iam_policy.permissions_boundary.arn

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect    = "Allow"
        Action    = "sts:AssumeRole"
        Principal = { Service = "lambda.amazonaws.com" }
      },
    ]
  })
}

# logs:CreateLogGroup を与えない。Log Group を消したときに Lambda が保持期間なしで作り直すと、保持期間の設定が失われるため
resource "aws_iam_policy" "lambda_logs" {
  name = "${local.lambda_function_name}-lambda-logs"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = "${aws_cloudwatch_log_group.lambda.arn}:*"
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_logs" {
  role       = aws_iam_role.lambda.name
  policy_arn = aws_iam_policy.lambda_logs.arn
}

# X-Ray に送るには、実行 Role に KMS の署名とログ以外の権限が必要になる
#trivy:ignore:AWS-0066
resource "aws_lambda_function" "gates" {
  function_name    = local.lambda_function_name
  role             = aws_iam_role.lambda.arn
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  handler          = "bootstrap"
  package_type     = "Zip"
  filename         = var.lambda_package_path
  source_code_hash = filebase64sha256(var.lambda_package_path)

  # Discovery と JWKS の最大 10 秒に、KMS と GitHub API の呼び出しを足しても余裕を残す
  timeout = 30
  # 実測前の仮の値
  memory_size                    = 256
  reserved_concurrent_executions = var.lambda_reserved_concurrency

  # Issuer と GitHub API Host は入れない。Lambda の設定を変更できる主体が検証の基準を差し替えられるようになるため
  environment {
    variables = {
      GATES_ALLOWED_OWNER_ID     = var.allowed_owner_id
      GATES_ALLOWED_OWNER_NAME   = var.allowed_owner_name
      GATES_ALLOWED_OWNER_TYPE   = var.allowed_owner_type
      GATES_GITHUB_APP_CLIENT_ID = var.github_app_client_id
      GATES_AUDIENCE             = var.gates_audience
      GATES_KMS_KEY_ARN          = aws_kms_external_key.signing.arn
    }
  }

  # 実行 Role では Log Group を作れないため、関数より先に作成する
  depends_on = [aws_cloudwatch_log_group.lambda]
}
