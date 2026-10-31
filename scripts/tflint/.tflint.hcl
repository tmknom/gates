# terraform ruleset は tflint に同梱され、aws ruleset は Dockerfile でイメージに入れる
plugin "terraform" {
  enabled = true
  preset  = "all"
}

# deep_check は AWS の認証情報とネットワークを必要とするため使わない
plugin "aws" {
  enabled    = true
  deep_check = false
}
