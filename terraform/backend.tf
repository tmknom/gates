terraform {
  # コンテナで terraform/ を読み取り専用にしたまま書き込めるよう、State を terraform/local/ に置く
  backend "local" {
    path = "local/terraform.tfstate"
  }

  # S3 バックエンドへ移行するときは、local と置き換える。バケット名、キー、リージョンは -backend-config で渡し、Git に入れない
  # backend "s3" {}
}
