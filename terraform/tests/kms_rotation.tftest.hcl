# 鍵の更新の各段階を、State を共有する run の並びで再現する
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

# 既存の鍵の ARN を、新しく作る鍵の ARN と違う値にする。移動せずに作り直す Plan では、ARN が適用まで決まらず、比較できない
run "existing_single_key" {
  state_key = "rotation"

  module {
    source = "./tests/testdata/single-key"
  }

  override_resource {
    target = aws_kms_external_key.signing
    values = {
      arn = "arn:aws:kms:ap-northeast-1:123456789012:key/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
    }
  }
}

run "existing_key_moves_into_the_keys_unchanged" {
  command   = plan
  state_key = "rotation"

  assert {
    condition     = aws_kms_external_key.signing["key-1"].arn == "arn:aws:kms:ap-northeast-1:123456789012:key/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
    error_message = "the existing key must become key-1 without being replaced"
  }

  assert {
    condition     = aws_lambda_function.gates.environment[0].variables["GATES_KMS_KEY_ARN"] == "arn:aws:kms:ap-northeast-1:123456789012:key/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
    error_message = "the function must keep signing with the existing key"
  }
}

run "single_key" {
  state_key = "rotation"
}

# trace:checks AC-14
run "adding_a_key_keeps_the_active_key" {
  command   = plan
  state_key = "rotation"

  variables {
    kms_keys = { key-1 = "RSA_2048", key-2 = "RSA_4096" }
  }

  assert {
    condition     = aws_kms_external_key.signing["key-1"].arn == "arn:aws:kms:ap-northeast-1:123456789012:key/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
    error_message = "adding a key must not replace the existing key"
  }

  assert {
    condition     = aws_kms_external_key.signing["key-1"].key_spec == "RSA_2048"
    error_message = "adding a key must not change the existing key"
  }

  assert {
    condition     = aws_kms_external_key.signing["key-2"].key_spec == "RSA_4096"
    error_message = "the new key must carry its own key spec"
  }

  assert {
    condition     = aws_kms_external_key.signing["key-2"].key_usage == "SIGN_VERIFY"
    error_message = "the new key must be a signing key"
  }

  assert {
    condition     = aws_kms_external_key.signing["key-2"].policy == aws_kms_external_key.signing["key-1"].policy
    error_message = "every key must have the same key policy"
  }

  assert {
    condition     = aws_lambda_function.gates.environment[0].variables["GATES_KMS_KEY_ARN"] == "arn:aws:kms:ap-northeast-1:123456789012:key/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
    error_message = "adding a key must not switch the function to it"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.lambda_sign.policy).Statement[0].Resource == "arn:aws:kms:ap-northeast-1:123456789012:key/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
    error_message = "adding a key must not switch the execution role policy to it"
  }
}

run "two_keys" {
  state_key = "rotation"

  variables {
    kms_keys = { key-1 = "RSA_2048", key-2 = "RSA_4096" }
  }
}

# trace:checks AC-14
run "switching_the_active_key_points_signing_to_it" {
  command   = plan
  state_key = "rotation"

  variables {
    kms_keys       = { key-1 = "RSA_2048", key-2 = "RSA_4096" }
    active_kms_key = "key-2"
  }

  assert {
    condition     = aws_lambda_function.gates.environment[0].variables["GATES_KMS_KEY_ARN"] == "arn:aws:kms:ap-northeast-1:123456789012:key/66666666-7777-8888-9999-000000000000"
    error_message = "the function must sign with the active key"
  }

  assert {
    condition     = jsondecode(aws_iam_policy.lambda_sign.policy).Statement[0].Resource == "arn:aws:kms:ap-northeast-1:123456789012:key/66666666-7777-8888-9999-000000000000"
    error_message = "the execution role policy must name the active key"
  }

  assert {
    condition     = output.kms_key_arn == "arn:aws:kms:ap-northeast-1:123456789012:key/66666666-7777-8888-9999-000000000000"
    error_message = "the output must be the ARN of the active key"
  }

  assert {
    condition     = aws_kms_external_key.signing["key-1"].arn == "arn:aws:kms:ap-northeast-1:123456789012:key/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
    error_message = "switching must keep the previous key for switching back"
  }
}
