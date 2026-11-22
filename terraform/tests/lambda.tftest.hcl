mock_provider "aws" {
  mock_data "aws_iam_openid_connect_provider" {
    defaults = {
      client_id_list = ["sts.amazonaws.com"]
    }
  }

  # 環境変数の合計の検査が数える KMS Key ARN の長さを、パーティションの名前で変えない
  mock_data "aws_partition" {
    defaults = {
      partition = "aws"
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
  allowed_owner_id       = "12345678"
  allowed_owner_name     = "Example-Owner"
  allowed_owner_type     = "Organization"
  github_app_client_id   = "Iv23example"
  gates_audience         = "gates-test"
  kms_keys               = { key-1 = "RSA_2048" }
  active_kms_key         = "key-1"
  lambda_package_path    = "tests/testdata/package.txt"
  issuable_permissions   = ["contents:read", "contents:write", "pull_requests:write"]
  protected_repositories = "repositories: []\n"

  # 拒否の試験の基準にする、受理される一覧
  accepted_protected_repositories = <<-EOT
    repositories:
      # 共通基盤
      - id: "200"
        name: platform-infra
        callers:
          - repository_id: "100"
            repository_name: deploy-tools
            permissions:
              contents: write
              pull_requests: read
      # どの Caller にも許可しない
      - id: "201"
        name: audit-records
        callers: []
  EOT
}

# trace:checks AC-QUALITY-003
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
    condition     = aws_lambda_function.gates.timeout == 45
    error_message = "the timeout must be 45 seconds, longer than the total of the Call Time Budget"
  }

  assert {
    condition     = aws_lambda_function.gates.reserved_concurrent_executions == -1
    error_message = "the function must not reserve concurrency by default"
  }

  assert {
    condition = aws_lambda_function.gates.environment[0].variables == tomap({
      GATES_ALLOWED_OWNER_ID       = "12345678"
      GATES_ALLOWED_OWNER_NAME     = "Example-Owner"
      GATES_ALLOWED_OWNER_TYPE     = "Organization"
      GATES_GITHUB_APP_CLIENT_ID   = "Iv23example"
      GATES_AUDIENCE               = "gates-test"
      GATES_KMS_KEY_ARN            = "arn:aws:kms:ap-northeast-1:123456789012:key/11111111-2222-3333-4444-555555555555"
      GATES_ISSUABLE_PERMISSIONS   = "contents:read,contents:write,pull_requests:write"
      GATES_PROTECTED_REPOSITORIES = "repositories: []\n"
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

# trace:checks AC-OPS-005
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

run "rejects_gates_audience_of_aws_sts" {
  command = plan

  variables {
    gates_audience = "sts.amazonaws.com"
  }

  expect_failures = [var.gates_audience]
}

run "rejects_gates_audience_of_github_default" {
  command = plan

  variables {
    gates_audience = "https://github.com/Example-Owner"
  }

  expect_failures = [var.gates_audience]
}

run "rejects_gates_audience_of_github_host" {
  command = plan

  variables {
    gates_audience = "https://github.com"
  }

  expect_failures = [var.gates_audience]
}

run "rejects_gates_audience_of_github_host_in_other_case" {
  command = plan

  variables {
    gates_audience = "HTTPS://GitHub.COM"
  }

  expect_failures = [var.gates_audience]
}

run "rejects_gates_audience_of_github_host_with_trailing_slash" {
  command = plan

  variables {
    gates_audience = "https://github.com/"
  }

  expect_failures = [var.gates_audience]
}

run "rejects_gates_audience_of_github_repository" {
  command = plan

  variables {
    gates_audience = "https://github.com/Example-Owner/gates"
  }

  expect_failures = [var.gates_audience]
}

run "rejects_gates_audience_of_github_repository_with_uppercase_host" {
  command = plan

  variables {
    gates_audience = "https://GitHub.com/Example-Owner/gates"
  }

  expect_failures = [var.gates_audience]
}

run "accepts_gates_audience_of_non_github_url" {
  command = plan

  variables {
    gates_audience = "https://gates.example.com"
  }

  assert {
    condition     = aws_lambda_function.gates.environment[0].variables["GATES_AUDIENCE"] == "https://gates.example.com"
    error_message = "the audience must carry the input value"
  }
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

# trace:checks AC-AUTHZ-010
run "rejects_empty_issuable_permissions" {
  command = plan

  variables {
    issuable_permissions = []
  }

  expect_failures = [var.issuable_permissions]
}

run "rejects_issuable_permission_without_colon" {
  command = plan

  variables {
    issuable_permissions = ["contentsread"]
  }

  expect_failures = [var.issuable_permissions]
}

run "rejects_issuable_permission_with_two_colons" {
  command = plan

  variables {
    issuable_permissions = ["contents:read:write"]
  }

  expect_failures = [var.issuable_permissions]
}

run "rejects_issuable_permission_with_empty_name" {
  command = plan

  variables {
    issuable_permissions = [":read"]
  }

  expect_failures = [var.issuable_permissions]
}

run "rejects_issuable_permission_with_empty_level" {
  command = plan

  variables {
    issuable_permissions = ["contents:"]
  }

  expect_failures = [var.issuable_permissions]
}

run "rejects_issuable_permission_with_too_long_name" {
  command = plan

  variables {
    issuable_permissions = ["${join("", [for i in range(65) : "a"])}:read"]
  }

  expect_failures = [var.issuable_permissions]
}

run "rejects_issuable_permission_with_uppercase" {
  command = plan

  variables {
    issuable_permissions = ["Contents:read"]
  }

  expect_failures = [var.issuable_permissions]
}

run "rejects_issuable_permission_with_surrounding_whitespace" {
  command = plan

  variables {
    issuable_permissions = [" contents:read"]
  }

  expect_failures = [var.issuable_permissions]
}

run "rejects_issuable_permission_with_trailing_newline" {
  command = plan

  variables {
    issuable_permissions = ["contents:read\n"]
  }

  expect_failures = [var.issuable_permissions]
}

run "rejects_duplicate_issuable_permissions" {
  command = plan

  variables {
    issuable_permissions = ["contents:read", "contents:read"]
  }

  expect_failures = [var.issuable_permissions]
}

run "protected_repositories_is_passed_as_written" {
  command = plan

  variables {
    protected_repositories = var.accepted_protected_repositories
  }

  assert {
    condition     = aws_lambda_function.gates.environment[0].variables["GATES_PROTECTED_REPOSITORIES"] == "repositories:\n  # 共通基盤\n  - id: \"200\"\n    name: platform-infra\n    callers:\n      - repository_id: \"100\"\n        repository_name: deploy-tools\n        permissions:\n          contents: write\n          pull_requests: read\n  # どの Caller にも許可しない\n  - id: \"201\"\n    name: audit-records\n    callers: []\n"
    error_message = "the protected repositories must be passed to the function as written, including the comments"
  }
}

run "accepts_same_caller_for_different_protected_repositories" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "    callers: []\n", "    callers:\n      - repository_id: \"100\"\n        repository_name: deploy-tools\n        permissions:\n          contents: read\n")
  }

  assert {
    condition     = strcontains(aws_lambda_function.gates.environment[0].variables["GATES_PROTECTED_REPOSITORIES"], "contents: read")
    error_message = "the protected repositories must be passed to the function"
  }
}

run "accepts_longest_protected_repository_id_and_name" {
  command = plan

  variables {
    protected_repositories = replace(replace(var.accepted_protected_repositories, "\"201\"", "\"999999999999999999\""), "audit-records", join("", [for i in range(100) : "a"]))
  }

  assert {
    condition     = strcontains(aws_lambda_function.gates.environment[0].variables["GATES_PROTECTED_REPOSITORIES"], "999999999999999999")
    error_message = "the protected repositories must be passed to the function"
  }
}

# trace:checks AC-AUTHZ-013
run "rejects_protected_repositories_that_is_not_yaml" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "callers: []", "callers: [")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_empty_protected_repositories" {
  command = plan

  variables {
    protected_repositories = ""
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_protected_repositories_with_two_documents" {
  command = plan

  variables {
    protected_repositories = "${var.accepted_protected_repositories}---\nrepositories: []\n"
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_protected_repositories_with_unknown_top_level_key" {
  command = plan

  variables {
    protected_repositories = "version: \"1\"\n${var.accepted_protected_repositories}"
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_protected_repositories_that_is_a_mapping" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "repositories:\n", "repositories:\n  platform:\n")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_protected_repository_without_name" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "    name: audit-records\n", "")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_protected_repository_with_unknown_key" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "    name: audit-records\n", "    name: audit-records\n    owner: example-owner\n")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_callers_that_is_a_mapping" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "callers: []", "callers: {}")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_caller_without_permissions" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "        permissions:\n          contents: write\n          pull_requests: read\n", "")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_caller_with_empty_permissions" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "        permissions:\n          contents: write\n          pull_requests: read\n", "        permissions: {}\n")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_zero_protected_repository_id" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "\"201\"", "\"0\"")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_protected_repository_id_with_leading_zero" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "\"201\"", "\"01\"")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_caller_repository_id_that_may_overflow_int64" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "\"100\"", "\"1000000000000000000\"")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_unquoted_protected_repository_id" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "\"201\"", "201")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_protected_repository_name_with_owner_name" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "name: audit-records", "name: example-owner/audit-records")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_too_long_caller_repository_name" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "deploy-tools", join("", [for i in range(101) : "a"]))
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_uppercase_permission_name" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "contents: write", "Contents: write")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_permission_level_outside_character_set" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "pull_requests: read", "pull_requests: read-only")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_duplicate_protected_repository_ids" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "\"201\"", "\"200\"")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_protected_repository_names_that_differ_only_in_case" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "audit-records", "Platform-Infra")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_duplicate_caller_repository_ids_in_protected_repository" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "          pull_requests: read\n", "          pull_requests: read\n      - repository_id: \"100\"\n        repository_name: release-tools\n        permissions:\n          contents: read\n")
  }

  expect_failures = [var.protected_repositories]
}

run "rejects_duplicate_caller_repository_names_in_protected_repository" {
  command = plan

  variables {
    protected_repositories = replace(var.accepted_protected_repositories, "          pull_requests: read\n", "          pull_requests: read\n      - repository_id: \"101\"\n        repository_name: Deploy-Tools\n        permissions:\n          contents: read\n")
  }

  expect_failures = [var.protected_repositories]
}

# 一覧が "repositories: []\n#\n" のとき、この試験の変数での合計は 433 バイトで、# の後の ASCII の 1 文字が 1 バイトを足す
run "accepts_environment_variables_of_4096_bytes" {
  command = plan

  variables {
    protected_repositories = "repositories: []\n#${replace(format("%3663s", ""), " ", "a")}\n"
  }

  assert {
    condition     = length(aws_lambda_function.gates.environment[0].variables["GATES_PROTECTED_REPOSITORIES"]) == 3682
    error_message = "the protected repositories must be passed to the function"
  }
}

run "rejects_environment_variables_over_4096_bytes" {
  command = plan

  variables {
    protected_repositories = "repositories: []\n#${replace(format("%3664s", ""), " ", "a")}\n"
  }

  expect_failures = [aws_lambda_function.gates]
}

# 文字数は 4096 以下で、UTF-8 のバイト数は 4096 を超える
run "rejects_environment_variables_over_4096_bytes_in_utf8" {
  command = plan

  variables {
    protected_repositories = "repositories: []\n#${replace(format("%1300s", ""), " ", "あ")}\n"
  }

  expect_failures = [aws_lambda_function.gates]
}
