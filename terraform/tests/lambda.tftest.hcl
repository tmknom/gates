mock_provider "aws" {
  mock_data "aws_iam_openid_connect_provider" {
    defaults = {
      client_id_list = ["sts.amazonaws.com"]
    }
  }

  mock_resource "aws_cloudwatch_log_group" {
    defaults = {
      arn = "arn:aws:logs:ap-northeast-1:123456789012:log-group:/aws/lambda/gates"
    }
  }

  mock_resource "aws_iam_role" {
    defaults = {
      arn = "arn:aws:iam::123456789012:role/gates-lambda"
    }
  }

  mock_resource "aws_iam_policy" {
    defaults = {
      arn = "arn:aws:iam::123456789012:policy/other"
    }
  }

  mock_resource "aws_kms_external_key" {
    defaults = {
      arn = "arn:aws:kms:ap-northeast-1:123456789012:key/11111111-2222-3333-4444-555555555555"
    }
  }
}

override_resource {
  target = aws_iam_policy.lambda_logs
  values = {
    arn = "arn:aws:iam::123456789012:policy/gates-lambda-logs"
  }
}

variables {
  allowed_owner_id     = "12345678"
  allowed_owner_name   = "Example-Owner"
  allowed_owner_type   = "Organization"
  github_app_client_id = "Iv23example"
  gates_audience       = "gates-test"
  kms_key_spec         = "RSA_2048"
  lambda_package_path  = "tests/testdata/package.txt"
}

run "function_configuration" {
  assert {
    condition     = aws_lambda_function.gates.function_name == "gates"
    error_message = "the function name must be gates"
  }

  assert {
    condition     = aws_lambda_function.gates.runtime == "provided.al2023"
    error_message = "the runtime must be provided.al2023"
  }

  assert {
    condition     = aws_lambda_function.gates.architectures == tolist(["arm64"])
    error_message = "the architecture must be arm64 only"
  }

  assert {
    condition     = aws_lambda_function.gates.handler == "bootstrap"
    error_message = "the handler must be bootstrap"
  }

  assert {
    condition     = aws_lambda_function.gates.reserved_concurrent_executions == -1
    error_message = "the function must not reserve concurrency by default"
  }

  assert {
    condition = aws_lambda_function.gates.environment[0].variables == tomap({
      GATES_ALLOWED_OWNER_ID     = "12345678"
      GATES_ALLOWED_OWNER_NAME   = "Example-Owner"
      GATES_ALLOWED_OWNER_TYPE   = "Organization"
      GATES_GITHUB_APP_CLIENT_ID = "Iv23example"
      GATES_AUDIENCE             = "gates-test"
      GATES_KMS_KEY_ARN          = "arn:aws:kms:ap-northeast-1:123456789012:key/11111111-2222-3333-4444-555555555555"
    })
    error_message = "the environment variables must carry the input values"
  }
}

run "log_group_configuration" {
  assert {
    condition     = aws_cloudwatch_log_group.lambda.name == "/aws/lambda/gates"
    error_message = "the log group must be the one Lambda writes to"
  }

  assert {
    condition     = aws_cloudwatch_log_group.lambda.retention_in_days == 7
    error_message = "the default retention must be 7 days"
  }
}

run "execution_role_policy_allows_only_writing_logs" {
  assert {
    condition     = length(jsondecode(aws_iam_policy.lambda_logs.policy).Statement) == 1
    error_message = "the policy must have one statement"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.lambda_logs.policy).Statement[0].Effect == "Allow"
    error_message = "the statement must be an allow"
  }

  assert {
    condition     = toset(jsondecode(aws_iam_policy.lambda_logs.policy).Statement[0].Action) == toset(["logs:CreateLogStream", "logs:PutLogEvents"])
    error_message = "the actions must be the two for writing logs"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.lambda_logs.policy).Statement[0].Resource == "arn:aws:logs:ap-northeast-1:123456789012:log-group:/aws/lambda/gates:*"
    error_message = "the resource must be the log group of the function"
  }
}

run "execution_role_policy_is_attached_to_execution_role" {
  assert {
    condition     = aws_iam_role_policy_attachment.lambda_logs.role == "gates-lambda"
    error_message = "the policy must be attached to the execution role"
  }

  assert {
    condition     = aws_iam_role_policy_attachment.lambda_logs.policy_arn == "arn:aws:iam::123456789012:policy/gates-lambda-logs"
    error_message = "the attached policy must be the one for writing logs"
  }
}

run "execution_role_trusts_only_lambda" {
  assert {
    condition     = length(jsondecode(aws_iam_role.lambda.assume_role_policy).Statement) == 1
    error_message = "the trust policy must have one statement"
  }

  assert {
    condition     = jsondecode(aws_iam_role.lambda.assume_role_policy).Statement[0].Principal == { Service = "lambda.amazonaws.com" }
    error_message = "the trusted principal must be the Lambda service only"
  }

  assert {
    condition     = jsondecode(aws_iam_role.lambda.assume_role_policy).Statement[0].Action == "sts:AssumeRole"
    error_message = "the trust policy must allow only sts:AssumeRole"
  }
}

run "rejects_zero_owner_id" {
  command = plan

  variables {
    allowed_owner_id = "0"
  }

  expect_failures = [var.allowed_owner_id]
}

run "rejects_non_numeric_owner_id" {
  command = plan

  variables {
    allowed_owner_id = "12a"
  }

  expect_failures = [var.allowed_owner_id]
}

run "rejects_owner_id_that_may_overflow_int64" {
  command = plan

  variables {
    allowed_owner_id = "1000000000000000000"
  }

  expect_failures = [var.allowed_owner_id]
}

run "rejects_owner_name_outside_character_set" {
  command = plan

  variables {
    allowed_owner_name = "owner_name"
  }

  expect_failures = [var.allowed_owner_name]
}

run "rejects_unknown_owner_type" {
  command = plan

  variables {
    allowed_owner_type = "enterprise"
  }

  expect_failures = [var.allowed_owner_type]
}

run "rejects_lowercase_owner_type" {
  command = plan

  variables {
    allowed_owner_type = "organization"
  }

  expect_failures = [var.allowed_owner_type]
}

run "rejects_github_app_client_id_with_surrounding_whitespace" {
  command = plan

  variables {
    github_app_client_id = "Iv23example "
  }

  expect_failures = [var.github_app_client_id]
}

run "rejects_gates_audience_with_surrounding_whitespace" {
  command = plan

  variables {
    gates_audience = "gates-test "
  }

  expect_failures = [var.gates_audience]
}

run "rejects_log_retention_unsupported_by_cloudwatch_logs" {
  command = plan

  variables {
    log_retention_in_days = 2
  }

  expect_failures = [var.log_retention_in_days]
}

run "rejects_reserved_concurrency_below_minus_one" {
  command = plan

  variables {
    lambda_reserved_concurrency = -2
  }

  expect_failures = [var.lambda_reserved_concurrency]
}

run "rejects_fractional_reserved_concurrency" {
  command = plan

  variables {
    lambda_reserved_concurrency = 1.5
  }

  expect_failures = [var.lambda_reserved_concurrency]
}

run "rejects_blank_package_path" {
  command = plan

  variables {
    lambda_package_path = " "
  }

  expect_failures = [var.lambda_package_path]
}

run "rejects_empty_github_app_client_id" {
  command = plan

  variables {
    github_app_client_id = ""
  }

  expect_failures = [var.github_app_client_id]
}

run "rejects_empty_gates_audience" {
  command = plan

  variables {
    gates_audience = ""
  }

  expect_failures = [var.gates_audience]
}
