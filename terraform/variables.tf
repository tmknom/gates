variable "region" {
  description = "リソースを作る AWS のリージョン。変えるときは scripts/terraform/proxy/allow-aws.txt も変更する"
  type        = string
  default     = "ap-northeast-1"
}
