# この URL の Provider はアカウントで一つだけの共有リソースのため、管理せず参照する。State に入れると、GATES の削除が他の用途の OIDC 連携を壊す
data "aws_iam_openid_connect_provider" "github_actions" {
  url = "https://token.actions.githubusercontent.com"

  # 管理外のリソースは Client ID を保証しない。Trust Policy の aud と食い違うと、引き受けの失敗まで気づけない
  lifecycle {
    postcondition {
      condition     = contains(self.client_id_list, "sts.amazonaws.com")
      error_message = "the IAM OIDC provider must list sts.amazonaws.com as a client ID"
    }
  }
}

# sub の先頭をオーナー名と不変 Owner ID の組に固定し、名前だけの旧形式の sub を一致させない
resource "aws_iam_role" "github_actions" {
  name                 = "${local.lambda_function_name}-github-actions"
  permissions_boundary = var.iam_permissions_boundary_arn

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect    = "Allow"
        Action    = "sts:AssumeRoleWithWebIdentity"
        Principal = { Federated = data.aws_iam_openid_connect_provider.github_actions.arn }
        Condition = {
          StringEquals = { "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com" }
          StringLike   = { "token.actions.githubusercontent.com:sub" = "repo:${var.allowed_owner_name}@${var.allowed_owner_id}/*" }
        }
      },
    ]
  })
}

# Resource に :* を付けない。GATES はバージョンもエイリアスも発行しないため
resource "aws_iam_policy" "github_actions_invoke" {
  name = "${local.lambda_function_name}-github-actions-invoke"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = "lambda:InvokeFunction"
        Resource = aws_lambda_function.gates.arn
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "github_actions_invoke" {
  role       = aws_iam_role.github_actions.name
  policy_arn = aws_iam_policy.github_actions_invoke.arn
}
