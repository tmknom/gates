# 名前を変えるときは、この名前を許可しているアカウント側の設定も変更する
# name、path、description の変更は置換になり、旧ポリシーの削除が先に走る。DeleteConflict で失敗したら、先に Role から Boundary を外す
# Resource はすべて Role の権限の上限を示す。実際の権限は各 Role のポリシーと Key Policy で絞る
resource "aws_iam_policy" "permissions_boundary" {
  name = "${local.lambda_function_name}-permissions-boundary"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      # 関数の ARN を参照しないのは、関数、実行 Role、Boundary の順に依存して循環するため
      {
        Effect   = "Allow"
        Action   = "lambda:InvokeFunction"
        Resource = "arn:${data.aws_partition.current.partition}:lambda:${var.region}:${data.aws_caller_identity.current.account_id}:function:${local.lambda_function_name}"
      },
      {
        Effect   = "Allow"
        Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = "${aws_cloudwatch_log_group.lambda.arn}:*"
      },
      # Key の ARN を参照しないのは、Key、実行 Role、Boundary の順に依存して循環するため。Key を参照しない Condition で対象を狭める
      {
        Effect   = "Allow"
        Action   = "kms:Sign"
        Resource = "arn:${data.aws_partition.current.partition}:kms:${var.region}:${data.aws_caller_identity.current.account_id}:key/*"
        Condition = {
          StringEquals = {
            "kms:KeyOrigin"        = "EXTERNAL"
            "kms:SigningAlgorithm" = "RSASSA_PKCS1_V1_5_SHA_256"
          }
        }
      },
    ]
  })
}
