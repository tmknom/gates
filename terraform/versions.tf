# scripts/terraform/Dockerfile はこのファイルと lock ファイルから Provider を取り込むため、バージョンの正本はここに置く
# バージョンを変えるときは、試験用の構成の tests/testdata/single-key/main.tf も変更する
terraform {
  required_version = "1.16.7"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.70.0"
    }
  }
}
