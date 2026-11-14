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

  mock_resource "aws_iam_policy" {
    defaults = {
      arn = "arn:aws:iam::123456789012:policy/other"
    }
  }
}

override_resource {
  target = aws_iam_policy.lambda_sign
  values = {
    arn = "arn:aws:iam::123456789012:policy/gates-lambda-sign"
  }
}

# 2 つ目の鍵の ARN を 1 つ目と違う値にし、どちらの鍵を参照しているかを見分ける
override_resource {
  target = aws_kms_external_key.signing["key-2"]
  values = {
    arn = "arn:aws:kms:ap-northeast-1:123456789012:key/66666666-7777-8888-9999-000000000000"
  }
}

variables {
  allowed_owner_id     = "12345678"
  allowed_owner_name   = "Example-Owner"
  allowed_owner_type   = "Organization"
  github_app_client_id = "Iv23example"
  gates_audience       = "gates-test"
  kms_keys             = { key-1 = "RSA_2048" }
  active_kms_key       = "key-1"
  lambda_package_path  = "tests/testdata/package.txt"
}

run "key_configuration" {
  assert {
    condition     = aws_kms_external_key.signing["key-1"].key_usage == "SIGN_VERIFY"
    error_message = "the key usage must be SIGN_VERIFY"
  }

  assert {
    condition     = aws_kms_external_key.signing["key-1"].key_spec == "RSA_2048"
    error_message = "the key spec must carry the input value"
  }

  assert {
    condition     = aws_kms_external_key.signing["key-1"].deletion_window_in_days == 30
    error_message = "the deletion window must be 30 days"
  }
}

# trace:checks AC-20
run "key_policy_allows_execution_role_only_to_sign" {
  assert {
    condition     = length(jsondecode(aws_kms_external_key.signing["key-1"].policy).Statement) == 2
    error_message = "the key policy must have two statements"
  }

  assert {
    condition     = jsondecode(aws_kms_external_key.signing["key-1"].policy).Statement[0].Effect == "Allow"
    error_message = "the signing statement must be an allow"
  }

  assert {
    condition     = jsondecode(aws_kms_external_key.signing["key-1"].policy).Statement[0].Resource == "*"
    error_message = "the signing statement must apply to this key only, which a key policy writes as *"
  }

  assert {
    condition     = jsondecode(aws_kms_external_key.signing["key-1"].policy).Statement[0].Principal == { AWS = "arn:aws:iam::123456789012:role/gates-lambda" }
    error_message = "the signing principal must be the execution role"
  }

  assert {
    condition     = jsondecode(aws_kms_external_key.signing["key-1"].policy).Statement[0].Action == "kms:Sign"
    error_message = "the execution role must be allowed only kms:Sign"
  }

  assert {
    condition     = jsondecode(aws_kms_external_key.signing["key-1"].policy).Statement[0].Condition == { StringEquals = { "kms:SigningAlgorithm" = "RSASSA_PKCS1_V1_5_SHA_256" } }
    error_message = "the signing algorithm must be restricted to RSASSA_PKCS1_V1_5_SHA_256"
  }
}

run "key_policy_allows_root_only_to_manage" {
  assert {
    condition     = jsondecode(aws_kms_external_key.signing["key-1"].policy).Statement[1].Effect == "Allow"
    error_message = "the management statement must be an allow"
  }

  assert {
    condition     = jsondecode(aws_kms_external_key.signing["key-1"].policy).Statement[1].Resource == "*"
    error_message = "the management statement must apply to this key only, which a key policy writes as *"
  }

  assert {
    condition     = jsondecode(aws_kms_external_key.signing["key-1"].policy).Statement[1].Principal == { AWS = "arn:aws:iam::123456789012:root" }
    error_message = "the management principal must be the account root"
  }

  assert {
    condition = toset(jsondecode(aws_kms_external_key.signing["key-1"].policy).Statement[1].Action) == toset([
      "kms:CancelKeyDeletion",
      "kms:DescribeKey",
      "kms:DisableKey",
      "kms:EnableKey",
      "kms:GetKeyPolicy",
      "kms:GetParametersForImport",
      "kms:GetPublicKey",
      "kms:ImportKeyMaterial",
      "kms:ListGrants",
      "kms:ListResourceTags",
      "kms:PutKeyPolicy",
      "kms:RevokeGrant",
      "kms:ScheduleKeyDeletion",
      "kms:TagResource",
      "kms:UntagResource",
      "kms:UpdateKeyDescription",
    ])
    error_message = "the management actions must match the list"
  }

  assert {
    condition     = !contains(jsondecode(aws_kms_external_key.signing["key-1"].policy).Statement[1].Action, "kms:Sign")
    error_message = "the root must not be allowed kms:Sign"
  }

  assert {
    condition     = !contains(jsondecode(aws_kms_external_key.signing["key-1"].policy).Statement[1].Action, "kms:*")
    error_message = "the root must not be allowed kms:*"
  }
}

# trace:checks AC-20
run "execution_role_policy_allows_only_signing_with_the_key" {
  assert {
    condition     = length(jsondecode(aws_iam_policy.lambda_sign.policy).Statement) == 1
    error_message = "the policy must have one statement"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.lambda_sign.policy).Statement[0].Effect == "Allow"
    error_message = "the statement must be an allow"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.lambda_sign.policy).Statement[0].Action == "kms:Sign"
    error_message = "the action must be kms:Sign only"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.lambda_sign.policy).Statement[0].Resource == "arn:aws:kms:ap-northeast-1:123456789012:key/11111111-2222-3333-4444-555555555555"
    error_message = "the resource must be the signing key"
  }
}

run "signing_policy_is_attached_to_execution_role" {
  assert {
    condition     = aws_iam_role_policy_attachment.lambda_sign.role == "gates-lambda"
    error_message = "the policy must be attached to the execution role"
  }

  assert {
    condition     = aws_iam_role_policy_attachment.lambda_sign.policy_arn == "arn:aws:iam::123456789012:policy/gates-lambda-sign"
    error_message = "the attached policy must be the one for signing"
  }
}

run "key_arns_are_output" {
  assert {
    condition     = output.kms_key_arn == "arn:aws:kms:ap-northeast-1:123456789012:key/11111111-2222-3333-4444-555555555555"
    error_message = "the output must be the ARN of the active key"
  }

  assert {
    condition     = output.kms_key_arns == { key-1 = "arn:aws:kms:ap-northeast-1:123456789012:key/11111111-2222-3333-4444-555555555555" }
    error_message = "the output must map each key name to its ARN"
  }
}

run "accepts_rsa_3072_key_spec" {
  command = plan

  variables {
    kms_keys = { key-1 = "RSA_3072" }
  }

  assert {
    condition     = aws_kms_external_key.signing["key-1"].key_spec == "RSA_3072"
    error_message = "the key spec must carry the input value"
  }
}

run "accepts_rsa_4096_key_spec" {
  command = plan

  variables {
    kms_keys = { key-1 = "RSA_4096" }
  }

  assert {
    condition     = aws_kms_external_key.signing["key-1"].key_spec == "RSA_4096"
    error_message = "the key spec must carry the input value"
  }
}

run "rejects_key_spec_that_is_not_rsa" {
  command = plan

  variables {
    kms_keys = { key-1 = "ECC_NIST_P256" }
  }

  expect_failures = [var.kms_keys]
}

run "rejects_rsa_key_spec_of_unsupported_length" {
  command = plan

  variables {
    kms_keys = { key-1 = "RSA_1024" }
  }

  expect_failures = [var.kms_keys]
}

run "rejects_no_keys" {
  command = plan

  variables {
    kms_keys = {}
  }

  expect_failures = [var.kms_keys]
}

# 実行 Role が署名できる鍵は、切り替えの前後の 2 つに限る
run "rejects_more_than_two_keys" {
  command = plan

  variables {
    kms_keys = { key-1 = "RSA_2048", key-2 = "RSA_2048", key-3 = "RSA_2048" }
  }

  expect_failures = [var.kms_keys]
}

run "rejects_key_name_outside_the_naming_rule" {
  command = plan

  variables {
    kms_keys       = { new = "RSA_2048" }
    active_kms_key = "new"
  }

  expect_failures = [var.kms_keys]
}

run "rejects_key_name_with_leading_zero" {
  command = plan

  variables {
    kms_keys       = { key-01 = "RSA_2048" }
    active_kms_key = "key-01"
  }

  expect_failures = [var.kms_keys]
}

run "rejects_active_key_outside_the_keys" {
  command = plan

  variables {
    active_kms_key = "key-2"
  }

  expect_failures = [var.active_kms_key]
}
