# 鍵の集合を導入する前の、KMS Key を 1 つだけ定義していた構成のアドレスを再現する。試験のための構成で、モジュールとして配布しない
terraform {
  required_version = "1.16.7"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.70.0"
    }
  }
}

resource "aws_kms_external_key" "signing" {
  key_usage = "SIGN_VERIFY"
  key_spec  = "RSA_2048"
}
