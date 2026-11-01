output "kms_key_arn" {
  description = "署名に使う KMS Key の ARN"
  value       = aws_kms_external_key.signing.arn
}
