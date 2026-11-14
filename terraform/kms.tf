data "aws_caller_identity" "current" {}

data "aws_partition" "current" {}

# 鍵の有効化の状態、鍵素材の有効期限、鍵素材は設定に書かない。手動インポートと競合するか、鍵素材が State に入るため
# インポートした鍵素材は交換できないため、鍵の更新は鍵の追加と有効な鍵の切り替えで行う。どの鍵も同じ Key Policy にする
resource "aws_kms_external_key" "signing" {
  for_each = var.kms_keys

  key_usage               = "SIGN_VERIFY"
  key_spec                = each.value
  deletion_window_in_days = 30

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "AllowSign"
        Effect    = "Allow"
        Principal = { AWS = aws_iam_role.lambda.arn }
        Action    = "kms:Sign"
        Resource  = "*"
        Condition = {
          StringEquals = { "kms:SigningAlgorithm" = "RSASSA_PKCS1_V1_5_SHA_256" }
        }
      },
      # 特定の Role を書くと、その Role を失ったときに鍵を管理できなくなるため、IAM ポリシーへ委任する
      # kms:* にしないのは、IAM の管理権限を持つ主体が Key Policy を変更せずに署名できるようになるため
      {
        Sid       = "AllowKeyManagement"
        Effect    = "Allow"
        Principal = { AWS = "arn:${data.aws_partition.current.partition}:iam::${data.aws_caller_identity.current.account_id}:root" }
        Action = [
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
        ]
        Resource = "*"
      },
    ]
  })

  # Map から鍵を除いた Plan も拒否する。退役させる鍵は、Map から除くのと同時に make tf-state-rm-kms-key で State から外す
  lifecycle {
    prevent_destroy = true
  }
}

# 鍵の集合を導入する前の、1 つだけの鍵を key-1 として引き継ぐ
moved {
  from = aws_kms_external_key.signing
  to   = aws_kms_external_key.signing["key-1"]
}

# 署名の可否は Key Policy が決める。このポリシーは Role の権限を IAM からも確認できるように明示し、有効な鍵だけを書く
# Role の中に書かず独立させることで、Role、Key、ポリシーの順に作れる
resource "aws_iam_policy" "lambda_sign" {
  name = "${local.lambda_function_name}-lambda-sign"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = "kms:Sign"
        Resource = aws_kms_external_key.signing[var.active_kms_key].arn
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_sign" {
  role       = aws_iam_role.lambda.name
  policy_arn = aws_iam_policy.lambda_sign.arn
}
