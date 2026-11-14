output "kms_key_arn" {
  description = "署名に使う有効な KMS Key の ARN"
  value       = aws_kms_external_key.signing[var.active_kms_key].arn
}

output "kms_key_arns" {
  description = "鍵の名前ごとの KMS Key の ARN。鍵素材をインポートする鍵を名前で選ぶ"
  value       = { for name, key in aws_kms_external_key.signing : name => key.arn }
}

output "github_actions_role_name" {
  description = "GitHub Actions 用 IAM Role の名前。接続情報の GATES_ROLE_NAME に登録する"
  value       = aws_iam_role.github_actions.name
}

output "lambda_function_name" {
  description = "Lambda の関数名。接続情報の GATES_FUNCTION_NAME に登録する"
  value       = aws_lambda_function.gates.function_name
}
