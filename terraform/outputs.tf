output "kms_key_arn" {
  description = "署名に使う KMS Key の ARN"
  value       = aws_kms_external_key.signing.arn
}

output "github_actions_role_name" {
  description = "GitHub Actions 用 IAM Role の名前。接続情報の GATES_ROLE_NAME に登録する"
  value       = aws_iam_role.github_actions.name
}

output "lambda_function_name" {
  description = "Lambda の関数名。接続情報の GATES_FUNCTION_NAME に登録する"
  value       = aws_lambda_function.gates.function_name
}
