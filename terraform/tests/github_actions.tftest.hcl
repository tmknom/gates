mock_provider "aws" {
  mock_data "aws_iam_openid_connect_provider" {
    defaults = {
      arn            = "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"
      client_id_list = ["sts.amazonaws.com"]
    }
  }

  mock_resource "aws_iam_policy" {
    defaults = {
      arn = "arn:aws:iam::123456789012:policy/other"
    }
  }

  mock_resource "aws_iam_role" {
    defaults = {
      arn = "arn:aws:iam::123456789012:role/gates-github-actions"
    }
  }

  mock_resource "aws_lambda_function" {
    defaults = {
      arn = "arn:aws:lambda:ap-northeast-1:123456789012:function:gates"
    }
  }
}

override_resource {
  target = aws_iam_policy.github_actions_invoke
  values = {
    arn = "arn:aws:iam::123456789012:policy/gates-github-actions-invoke"
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

run "provider_without_sts_client_id_is_rejected" {
  command = plan

  override_data {
    target = data.aws_iam_openid_connect_provider.github_actions
    values = {
      client_id_list = ["other-audience"]
    }
  }

  expect_failures = [data.aws_iam_openid_connect_provider.github_actions]
}

run "trust_policy_allows_only_web_identity_from_provider" {
  assert {
    condition     = length(jsondecode(aws_iam_role.github_actions.assume_role_policy).Statement) == 1
    error_message = "the trust policy must have one statement"
  }

  assert {
    condition     = jsondecode(aws_iam_role.github_actions.assume_role_policy).Statement[0].Effect == "Allow"
    error_message = "the statement must be an allow"
  }

  assert {
    condition     = jsondecode(aws_iam_role.github_actions.assume_role_policy).Statement[0].Action == "sts:AssumeRoleWithWebIdentity"
    error_message = "the trust policy must allow only sts:AssumeRoleWithWebIdentity"
  }

  assert {
    condition     = jsondecode(aws_iam_role.github_actions.assume_role_policy).Statement[0].Principal == { Federated = "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com" }
    error_message = "the trusted principal must be the referenced provider only"
  }
}

run "trust_policy_conditions_are_only_aud_and_sub" {
  assert {
    condition = jsondecode(aws_iam_role.github_actions.assume_role_policy).Statement[0].Condition == {
      StringEquals = { "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com" }
      StringLike   = { "token.actions.githubusercontent.com:sub" = "repo:Example-Owner@12345678/*" }
    }
    error_message = "the conditions must be the aud of AWS STS and the sub pattern with the immutable owner ID"
  }
}

run "invoke_policy_allows_only_invoking_the_function" {
  assert {
    condition     = length(jsondecode(aws_iam_policy.github_actions_invoke.policy).Statement) == 1
    error_message = "the policy must have one statement"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.github_actions_invoke.policy).Statement[0].Effect == "Allow"
    error_message = "the statement must be an allow"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.github_actions_invoke.policy).Statement[0].Action == "lambda:InvokeFunction"
    error_message = "the action must be lambda:InvokeFunction only"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.github_actions_invoke.policy).Statement[0].Resource == "arn:aws:lambda:ap-northeast-1:123456789012:function:gates"
    error_message = "the resource must be the function ARN without versions and aliases"
  }
}

run "invoke_policy_is_attached_to_role" {
  assert {
    condition     = aws_iam_role_policy_attachment.github_actions_invoke.role == "gates-github-actions"
    error_message = "the policy must be attached to the GitHub Actions role"
  }

  assert {
    condition     = aws_iam_role_policy_attachment.github_actions_invoke.policy_arn == "arn:aws:iam::123456789012:policy/gates-github-actions-invoke"
    error_message = "the attached policy must be the one for invoking the function"
  }
}

run "connection_values_are_output" {
  assert {
    condition     = output.github_actions_role_name == "gates-github-actions"
    error_message = "the output must be the role name"
  }

  assert {
    condition     = output.lambda_function_name == "gates"
    error_message = "the output must be the function name"
  }
}
