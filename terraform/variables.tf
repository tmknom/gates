variable "region" {
  description = "リソースを作る AWS のリージョン。変えるときは scripts/terraform/proxy/allow-aws.txt も変更する"
  type        = string
  default     = "ap-northeast-1"
}

# number にしないのは、表記の変換を挟まないため。Lambda が int64 で読むため、必ず収まる 18 桁までに限る
variable "allowed_owner_id" {
  description = "許可オーナーの不変 Owner ID"
  type        = string

  validation {
    condition     = can(regex("^[1-9][0-9]{0,17}$", var.allowed_owner_id))
    error_message = "allowed_owner_id must be a positive integer of up to 18 digits without leading zeros."
  }
}

# IAM Role の Trust Policy の StringLike は大文字小文字を区別する
variable "allowed_owner_name" {
  description = "許可オーナーの名前。GitHub 上の表記（大文字小文字）のまま書く"
  type        = string

  validation {
    condition     = can(regex("^[A-Za-z0-9-]+$", var.allowed_owner_name))
    error_message = "allowed_owner_name must contain only ASCII letters, digits and hyphens."
  }
}

variable "allowed_owner_type" {
  description = "許可オーナーの種別。Organization または User"
  type        = string

  validation {
    condition     = contains(["Organization", "User"], var.allowed_owner_type)
    error_message = "allowed_owner_type must be Organization or User."
  }
}

variable "github_app_client_id" {
  description = "GitHub App の Client ID"
  type        = string

  validation {
    condition     = var.github_app_client_id != "" && var.github_app_client_id == trimspace(var.github_app_client_id)
    error_message = "github_app_client_id must not be empty or have surrounding whitespace."
  }
}

variable "gates_audience" {
  description = "GATES 用 Audience"
  type        = string

  validation {
    condition     = var.gates_audience != "" && var.gates_audience == trimspace(var.gates_audience)
    error_message = "gates_audience must not be empty or have surrounding whitespace."
  }

  # AWS 用 OIDC JWT と同じ Audience にすると、AWS 用 OIDC JWT で GATES の認証を通せるようになる
  validation {
    condition     = var.gates_audience != "sts.amazonaws.com"
    error_message = "gates_audience must differ from sts.amazonaws.com, the audience of the OIDC JWT for AWS."
  }

  # GitHub は GitHub 全体を指す汎用の https://github.com でも OIDC JWT を発行する。同じ値にすると、同じ値を Audience に使うほかのサービス向けの OIDC JWT で GATES の認証を通せる
  validation {
    condition     = lower(var.gates_audience) != "https://github.com"
    error_message = "gates_audience must differ from https://github.com, which does not identify GATES."
  }

  # GitHub は呼び出し元のオーナーとリポジトリの URL でも OIDC JWT を発行し、オーナーの URL は Audience を指定しない OIDC JWT にも入る。同じ値にすると、その OIDC JWT で GATES の認証を通せる
  validation {
    condition     = !startswith(lower(var.gates_audience), "https://github.com/")
    error_message = "gates_audience must not start with https://github.com/, which does not identify GATES."
  }
}

# 既定値を持たせないのは、GitHub が Private Key の鍵長を文書で約束しておらず、見込みの鍵長を運用者に選ばせるため
variable "kms_keys" {
  description = "KMS Key の名前から key_spec への Map。key_spec は、鍵を適用の後に生成するため、インポートする鍵の見込みの鍵長に合わせる"
  type        = map(string)

  validation {
    condition     = alltrue([for key_spec in values(var.kms_keys) : contains(["RSA_2048", "RSA_3072", "RSA_4096"], key_spec)])
    error_message = "every key_spec in kms_keys must be RSA_2048, RSA_3072 or RSA_4096."
  }

  # 実行 Role が署名できる鍵を、切り替えの前後の 2 つまでに限る
  validation {
    condition     = length(var.kms_keys) >= 1 && length(var.kms_keys) <= 2
    error_message = "kms_keys must have one or two keys."
  }

  # 名前は State のアドレスに入り、make -C terraform state-rm-kms-key の引数にもなるため、形を固定する
  validation {
    condition     = alltrue([for name in keys(var.kms_keys) : can(regex("^key-[1-9][0-9]{0,8}$", name))])
    error_message = "every name in kms_keys must be key-N, where N is a positive integer without leading zeros."
  }
}

variable "active_kms_key" {
  description = "Lambda が署名に使う KMS Key の、kms_keys での名前"
  type        = string

  validation {
    condition     = contains(keys(var.kms_keys), var.active_kms_key)
    error_message = "active_kms_key must be a name in kms_keys."
  }
}

variable "log_retention_in_days" {
  description = "Lambda のログの保持期間（日）。組織の監査方針に合わせて変更する"
  type        = number
  default     = 7

  validation {
    condition     = contains([1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1096, 1827, 2192, 2557, 2922, 3288, 3653], var.log_retention_in_days)
    error_message = "log_retention_in_days must be a retention period supported by CloudWatch Logs."
  }
}

# 既定値で予約しないのは、予約すると未予約の同時実行数が下限を割るアカウントで Apply が失敗するため
variable "lambda_reserved_concurrency" {
  description = "Lambda の Reserved Concurrency。-1 は予約しない。0 はすべての Invoke を止める"
  type        = number
  default     = -1

  validation {
    condition     = var.lambda_reserved_concurrency >= -1 && floor(var.lambda_reserved_concurrency) == var.lambda_reserved_concurrency
    error_message = "lambda_reserved_concurrency must be an integer greater than or equal to -1."
  }
}

# チェックサムで確かめるパッケージと揃えるため、tfvars には書かず、terraform/Makefile の plan で渡す
variable "lambda_package_path" {
  description = "Lambda のデプロイパッケージの terraform/ からの相対パス"
  type        = string

  validation {
    condition     = trimspace(var.lambda_package_path) != ""
    error_message = "lambda_package_path must not be empty."
  }
}

# 許可権限に含まれるかは Terraform では確かめず、Lambda の初期化が確かめる。AWS Infrastructure Design を参照する
variable "issuable_permissions" {
  description = "発行上限。発行する権限の上限を「権限名:水準」の形式で並べたリスト"
  type        = list(string)

  validation {
    condition     = length(var.issuable_permissions) >= 1
    error_message = "issuable_permissions must have at least one element."
  }

  validation {
    condition     = alltrue([for permission in var.issuable_permissions : can(regex("^[a-z_]{1,64}:[a-z_]{1,64}$", permission))])
    error_message = "every element of issuable_permissions must be name:level, where each of name and level is 1 to 64 characters of ASCII lowercase letters and underscores."
  }

  validation {
    condition     = length(distinct(var.issuable_permissions)) == length(var.issuable_permissions)
    error_message = "issuable_permissions must not have duplicate elements."
  }
}

# sensitive にしないのは、資格情報ではなく、Plan の差分で一覧の変更をレビューするため
# 検証は locals から復号した値を読むと循環するため、各検証で復号する。前提の形が崩れた値は、式の誤りにせず try で拒否に倒す
variable "protected_repositories" {
  description = "保護対象の一覧。YAML の文書 1 件をヒアドキュメントで書き、保護対象を置かないデプロイは repositories: [] を書く。値はコメントを含めてそのまま Lambda の環境変数に入る。重複したキー、アンカー、明示的なタグ、%TAG、BOM、文字列以外に解決される引用符のない名前や権限、許可権限に含まれない組などは、Plan を通っても Lambda の初期化が拒否する。AWS Infrastructure Design を参照する"
  type        = string

  validation {
    condition     = try(keys(yamldecode(var.protected_repositories)) == ["repositories"] && startswith(jsonencode(yamldecode(var.protected_repositories).repositories), "["), false)
    error_message = "protected_repositories must be one YAML document that is a mapping with only the key repositories, whose value is a sequence."
  }

  validation {
    condition = try(alltrue([
      for repository in yamldecode(var.protected_repositories).repositories :
      keys(repository) == ["callers", "id", "name"] && startswith(jsonencode(repository.callers), "[")
    ]), false)
    error_message = "every protected repository in protected_repositories must have only the keys id, name and callers, and callers must be a sequence."
  }

  validation {
    condition = try(alltrue(flatten([
      for repository in yamldecode(var.protected_repositories).repositories : [
        for caller in repository.callers :
        keys(caller) == ["permissions", "repository_id", "repository_name"] && startswith(jsonencode(caller.permissions), "{") && length(keys(caller.permissions)) >= 1
      ]
    ])), false)
    error_message = "every caller in protected_repositories must have only the keys repository_id, repository_name and permissions, and permissions must be a mapping with at least one pair."
  }

  # Lambda が int64 で読むため、必ず収まる 18 桁までに限る
  validation {
    condition = try(alltrue(flatten([
      for repository in yamldecode(var.protected_repositories).repositories : [
        for id in concat([repository.id], [for caller in repository.callers : caller.repository_id]) :
        can(regex("^[1-9][0-9]{0,17}$", id))
      ]
    ])), false)
    error_message = "every id and repository_id in protected_repositories must be a positive integer of up to 18 digits without leading zeros."
  }

  # yamldecode は引用符のない ID を数値に復号し、jsonencode は数値を引用符なしで符号化する
  validation {
    condition = try(alltrue(flatten([
      for repository in yamldecode(var.protected_repositories).repositories : [
        for id in concat([repository.id], [for caller in repository.callers : caller.repository_id]) :
        startswith(jsonencode(id), "\"")
      ]
    ])), false)
    error_message = "every id and repository_id in protected_repositories must be quoted."
  }

  validation {
    condition = try(alltrue(flatten([
      for repository in yamldecode(var.protected_repositories).repositories : [
        for name in concat([repository.name], [for caller in repository.callers : caller.repository_name]) :
        can(regex("^[A-Za-z0-9._-]{1,100}$", name))
      ]
    ])), false)
    error_message = "every name and repository_name in protected_repositories must be 1 to 100 characters of ASCII letters, digits, hyphens, underscores and periods, without the owner name."
  }

  validation {
    condition = try(alltrue(flatten([
      for repository in yamldecode(var.protected_repositories).repositories : [
        for caller in repository.callers : [
          for name, level in caller.permissions :
          can(regex("^[a-z_]{1,64}$", name)) && can(regex("^[a-z_]{1,64}$", level))
        ]
      ]
    ])), false)
    error_message = "every permission name and level in protected_repositories must be 1 to 64 characters of ASCII lowercase letters and underscores."
  }

  # 名前は、Lambda の重複の判定（RULE-005）と同じく大文字小文字を区別せずに比べる
  validation {
    condition = try(
      length(distinct([for repository in yamldecode(var.protected_repositories).repositories : repository.id])) == length(yamldecode(var.protected_repositories).repositories)
      && length(distinct([for repository in yamldecode(var.protected_repositories).repositories : lower(repository.name)])) == length(yamldecode(var.protected_repositories).repositories),
      false
    )
    error_message = "protected repositories in protected_repositories must not share an id or a name, compared case-insensitively."
  }

  validation {
    condition = try(alltrue([
      for repository in yamldecode(var.protected_repositories).repositories :
      length(distinct([for caller in repository.callers : caller.repository_id])) == length(repository.callers)
      && length(distinct([for caller in repository.callers : lower(caller.repository_name)])) == length(repository.callers)
    ]), false)
    error_message = "callers of a protected repository in protected_repositories must not share a repository_id or a repository_name, compared case-insensitively."
  }
}

# 既定値と、型のほかの validation を置かない理由は AWS Infrastructure Design にある
variable "repository_policy_enforcement" {
  description = "リポジトリポリシーの適用。true は適用し、false は適用しない。適用するデプロイでは、GitHub App に Single file を Read-only で付与し、パスを .github/gates-policy.yml だけにする"
  type        = bool
  nullable    = false
}
