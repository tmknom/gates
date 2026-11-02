mock_provider "aws" {
  mock_data "aws_caller_identity" {
    defaults = {
      account_id = "123456789012"
    }
  }

  mock_data "aws_partition" {
    defaults = {
      partition = "aws"
    }
  }

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

  mock_resource "aws_iam_policy" {
    defaults = {
      arn = "arn:aws:iam::123456789012:policy/other"
    }
  }

  mock_resource "aws_iam_role" {
    defaults = {
      arn = "arn:aws:iam::123456789012:role/gates-lambda"
    }
  }

  mock_resource "aws_kms_external_key" {
    defaults = {
      arn = "arn:aws:kms:ap-northeast-1:123456789012:key/11111111-2222-3333-4444-555555555555"
    }
  }
}

override_resource {
  target = aws_iam_policy.permissions_boundary
  values = {
    arn = "arn:aws:iam::123456789012:policy/gates-permissions-boundary"
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

run "boundary_name_is_the_one_accounts_allow" {
  assert {
    condition     = aws_iam_policy.permissions_boundary.name == "gates-permissions-boundary"
    error_message = "the name must match the one allowed on the account side"
  }
}

run "boundary_allows_only_the_operations_of_the_roles" {
  assert {
    condition     = length(jsondecode(aws_iam_policy.permissions_boundary.policy).Statement) == 3
    error_message = "the boundary must have three statements"
  }

  assert {
    condition     = alltrue([for statement in jsondecode(aws_iam_policy.permissions_boundary.policy).Statement : statement.Effect == "Allow"])
    error_message = "every statement must be an allow"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.permissions_boundary.policy).Statement[0].Action == "lambda:InvokeFunction"
    error_message = "the first statement must allow lambda:InvokeFunction only"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.permissions_boundary.policy).Statement[0].Resource == "arn:aws:lambda:ap-northeast-1:123456789012:function:gates"
    error_message = "lambda:InvokeFunction must be limited to the function"
  }

  assert {
    condition     = toset(jsondecode(aws_iam_policy.permissions_boundary.policy).Statement[1].Action) == toset(["logs:CreateLogStream", "logs:PutLogEvents"])
    error_message = "the second statement must allow the two actions for writing logs"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.permissions_boundary.policy).Statement[1].Resource == "arn:aws:logs:ap-northeast-1:123456789012:log-group:/aws/lambda/gates:*"
    error_message = "writing logs must be limited to the log group of the function"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.permissions_boundary.policy).Statement[2].Action == "kms:Sign"
    error_message = "the third statement must allow kms:Sign only"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.permissions_boundary.policy).Statement[2].Resource == "arn:aws:kms:ap-northeast-1:123456789012:key/*"
    error_message = "kms:Sign must be limited to the keys in the region and the account"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.permissions_boundary.policy).Statement[2].Condition == { StringEquals = { "kms:KeyOrigin" = "EXTERNAL", "kms:SigningAlgorithm" = "RSASSA_PKCS1_V1_5_SHA_256" } }
    error_message = "kms:Sign must be limited to imported keys and the signing algorithm of the key policy"
  }
}

# Boundary が許可しないログの操作は、ポリシーに追加しても実行時に拒否される
run "boundary_covers_the_log_policy_of_the_execution_role" {
  assert {
    condition     = toset(jsondecode(aws_iam_policy.lambda_logs.policy).Statement[0].Action) == toset(jsondecode(aws_iam_policy.permissions_boundary.policy).Statement[1].Action)
    error_message = "the boundary must allow the same log actions as the policy of the execution role"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.lambda_logs.policy).Statement[0].Resource == jsondecode(aws_iam_policy.permissions_boundary.policy).Statement[1].Resource
    error_message = "the boundary must allow the same log resource as the policy of the execution role"
  }
}

run "boundary_follows_the_region" {
  variables {
    region = "us-east-1"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.permissions_boundary.policy).Statement[0].Resource == "arn:aws:lambda:us-east-1:123456789012:function:gates"
    error_message = "the function ARN must use the input region"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.permissions_boundary.policy).Statement[2].Resource == "arn:aws:kms:us-east-1:123456789012:key/*"
    error_message = "the key ARN must use the input region"
  }
}

run "boundary_is_attached_to_both_roles" {
  assert {
    condition     = aws_iam_role.lambda.permissions_boundary == "arn:aws:iam::123456789012:policy/gates-permissions-boundary"
    error_message = "the execution role must have the boundary"
  }

  assert {
    condition     = aws_iam_role.github_actions.permissions_boundary == "arn:aws:iam::123456789012:policy/gates-permissions-boundary"
    error_message = "the GitHub Actions role must have the boundary"
  }
}
