# scripts/terraform/Dockerfile はこのファイルと lock ファイルから Provider を取り込むため、バージョンの正本はここに置く
terraform {
  required_version = "1.16.7"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.70.0"
    }
  }
}
