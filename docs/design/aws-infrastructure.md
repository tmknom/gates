# AWS Infrastructure Design

## 1. Scope and Responsibility

AWS のリソース設定は Terraform で管理します。GitHub App Private Key の取得、変換、KMS への鍵素材インポートは Terraform の対象外とし、独立した運用手順に従います。

AWS 側は GitHub OIDC → AWS STS → IAM 認証付き Lambda 直接 Invoke → KMS `Sign` を提供します。GitHub の OIDC JWT によるリポジトリ単位の本人確認と GitHub App Token の発行認可は、Lambda 内の Go アプリケーションが行います。

Terraform の State / Plan、Git、CI の成果物、Lambda の環境変数に GitHub App Private Key や変換途中の秘密鍵素材を含めません。鍵素材を読み込む Terraform の Resource、Data Source、Provisioner、外部実行処理は作りません。

## 2. Resource Model

| 対象 | 構成・責務 | 設計上の制約 |
|---|---|---|
| IAM OIDC Provider | `https://token.actions.githubusercontent.com` を AWS STS の信頼先として設定 | 対象 AWS アカウントに既存の Provider がある場合は再利用し、重複作成しない |
| GitHub Actions IAM Role | 許可 Owner のワークフローに `AssumeRoleWithWebIdentity` を許可 | Trust Policy で Audience と不変 ID 入りの `sub` を制限。Permissions Policy は対象 Lambda の `lambda:InvokeFunction` に限定 |
| Lambda 実行 IAM Role | Go アプリケーションから必要な AWS API を呼び出す | 対象 KMS Key への `kms:Sign` と必要な CloudWatch Logs 操作のみを基本とする。`kms:ImportKeyMaterial` 等の鍵管理操作は付与しない |
| Lambda Function | JWT 検証、発行認可、KMS 署名依頼、GitHub API 通信 | Go の `provided.al2023` と ZIP 形式で配布し、IAM 認証付き Lambda Invoke API からだけ呼び出す。Function URL、API Gateway などの別の呼び出し口を設けず、他アカウントや匿名主体を許可するリソースベースポリシーを付与しない。利用者管理 VPC には接続しない |
| KMS Key | GitHub App Private Key（RSA）の鍵素材を外部からインポートするための署名鍵のメタデータを管理 | `origin=EXTERNAL`、`key_usage=SIGN_VERIFY`、実鍵長に一致する `key_spec=RSA_*`。インポート処理は Terraform の対象外 |
| KMS Key Policy | 鍵の管理・利用主体を制限 | 署名権限と鍵管理権限を分離。Lambda 実行 Role に署名権限のみ許可し、鍵の管理権限は運用者の管理主体に残す |
| KMS Alias（任意） | 運用者が署名鍵を識別するための補助 | Lambda と IAM は明示的な Key ARN を使用する。Alias を追加しても署名鍵は自動切り替えしない |
| CloudWatch Logs | Lambda の実行結果と監査情報を保管 | 保持期間とアクセス制限を設定。JWT、App JWT、Installation Token、AWS 一時認証情報は記録しない |

GitHub App の作成とインストール対象は GitHub 側で管理し、Terraform は AWS リソースの構成を管理します。

## 3. IAM Design

### 3.1 GitHub Actions IAM Role

Trust Policy は `aud` と `sub` の条件で引き受けを制限します。AWS IAM は `repository_owner_id` などの GitHub 固有 Claim も条件キーとして評価できますが、これらの Claim は名前だけの旧形式の `sub` を持つトークンにも含まれるため、旧形式の拒否には使えません。不変 Owner ID の制約と旧形式の拒否は、不変 ID 入りの `sub` 形式 `repo:OWNER@OWNER-ID/REPO@REPO-ID:<実行文脈>` に対する次のパターンで表現します。

```json
{
  "StringEquals": {
    "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
  },
  "StringLike": {
    "token.actions.githubusercontent.com:sub": "repo:<OWNER-NAME>@<OWNER-ID>/*"
  }
}
```

パターンの先頭を Owner 名と不変 Owner ID の組 `repo:<OWNER-NAME>@<OWNER-ID>/` に固定するのは、名前だけの旧形式 `repo:<OWNER-NAME>/<REPO>:...` を一致させないためです。GitHub のユーザー名とリポジトリ名には `@` を使えないため、旧形式の `sub` がこの先頭部分に一致することはありません。Owner ID の前を `*` にした `repo:*@<OWNER-ID>/*` のようなパターンは採用しません。`StringLike` の `*` は `/` や `:` を含む任意の文字列に一致するため、他の Owner のワークフローがブランチ名に `@<OWNER-ID>/` を含めるだけで一致してしまうからです。

Owner 共通の入口として、`REPO@REPO-ID` 以降の実行文脈（`ref`、`pull_request`、`environment` など）は制限しません。Owner 名を条件に含めるため、Owner の名前変更後は Trust Policy の更新まで引き受けが拒否されます。Lambda の環境変数にも Owner 名を持つため、名前変更時は Trust Policy と環境変数を併せて更新します。更新までの拒否は Fail Closed の動作として許容します。

IAM Role の権限は、GATES 用の一つの Lambda を同期 Invoke するための `lambda:InvokeFunction` に限定します。`kms:Sign`、`iam:PassRole`、Lambda の設定変更、鍵のインポート・削除は付与しません。許可された Owner のワークフローは共通 IAM Role を利用します。

IAM は Owner 単位の入口制御です。Lambda での Caller 識別は、別途検証した GATES 用 OIDC JWT の Claim によって行います。

### 3.2 Lambda Execution Role

Lambda 実行 Role には、指定 KMS Key ARN に対する `kms:Sign` のみを暗号操作として許可します。ログ記録に必要な CloudWatch Logs 権限は対象 Log Group に限定できる範囲で制約します。KMS Key Policy でも実行 Role による署名が成立するよう許可し、IAM Policy だけで鍵の利用が許可されると想定しません。

KMS Key Policy の変更権限、鍵素材のインポート・削除権限、Alias の更新権限は Lambda 実行 Role へ付与しません。鍵管理を担う主体は Lambda と分離します。

### 3.3 Deployment Permissions

Terraform を適用する主体には、リソース作成・ポリシー更新・コード配布に必要な権限を与えます。KMS 鍵素材のインポートに必要な権限は通常の Terraform 適用から分離します。Terraform の適用主体に強い管理権限が必要になる点は、実行主体と変更レビューによって制御します。

Terraform は運用者の手元から適用し、CI からは Plan も Apply も実行しません。
CI に適用権限を与えると、IAM と KMS Key Policy を変更できる強い権限を CI に置くことになるためです。

State は当面、適用する運用者の手元に置きます（ローカルバックエンド）。
State に秘密鍵素材は入りませんが、IAM と Lambda の構成が入るため、Git には含めません。
S3 バックエンドの設定はコメントアウトした状態で置いておき、S3 バックエンドへ移行するまで State 用のバケットは作りません。

## 4. KMS Resource and Import Boundary

### 4.1 Key Resource

KMS Key は鍵素材を持たない `EXTERNAL` origin の非対称署名鍵として作成します。鍵素材の初回インポート前は `PendingImport` であり、そのままでは署名できません。

Terraform では、`hashicorp/aws` Provider の `aws_kms_external_key` で KMS Key を作成します。KMS Key Policy と鍵の状態変化を含むリソース設定を管理し、鍵素材のインポートは Terraform で実行しません。

`aws_kms_external_key` が非対称鍵の `key_spec` を受け付けるのは Provider v6.11.0 以降のため、Provider のバージョンはそれ以上に固定します。
IAM と Lambda の管理に同じ Provider を使うので、KMS Key のためだけに Cloud Control（`awscc`）Provider を加えることはしません。
`key_material_base64` は設定しません。
この引数は 256 bit の対称鍵素材を Terraform からインポートするためのもので、RSA の鍵素材には使えず、値は State に平文で保存されます。

鍵素材を手動でインポートして鍵の状態が `PendingImport` から `Enabled` に変わっても、`enabled` を設定に書かなければ Plan に差分は出ません。
`enabled` は、設定で省略すると AWS 側の値を読み取って State に取り込む属性だからです。
`valid_to` も設定に書かず、手動インポートでは鍵素材を無期限（`KEY_MATERIAL_DOES_NOT_EXPIRE`）にします。
手動インポートで有効期限を付けると、Terraform は `valid_to` の差分を鍵素材の再インポートで解消しようとしますが、鍵素材を設定に持たないため、その更新は成り立ちません。

KMS Key の `key_spec` は、RSA_2048 / RSA_3072 / RSA_4096 のうち GitHub App Private Key の RSA 鍵長と一致する値とします。非対称鍵には KMS の自動ローテーションとオンデマンドローテーションが対応しないため、鍵の更新は新しい KMS Key の作成で行います。

### 4.2 Terraform and Key Material Lifecycle

- Terraform は KMS Key の設定、Key Policy、IAM 権限の正本とする。Alias を採用する場合はその設定も管理する。
- 秘密鍵の取得、PKCS#8 への変換、インポート用ラッピング、`ImportKeyMaterial` は独立した手順で行う。インポート後、インポート前の秘密鍵のコピーは運用者の手元から削除する。
- 秘密鍵の平文、暗号化途中の鍵素材、インポート用トークンは Terraform の State / Plan に渡さない。
- 鍵素材のインポートで鍵の状態が `PendingImport` から `Enabled` に変わっても、Terraform の変更で鍵を意図せず再作成・無効化しない。
- 既存の KMS Key の置換・削除を避けるため `prevent_destroy` を設定する。`prevent_destroy` は鍵の削除・置換を含む Plan をエラーで拒否するが、リソースの設定自体を取り除いた場合の削除は防がない。Plan に鍵削除が現れた場合は適用を中止して確認する。
- インポートした非対称 KMS Key の鍵素材は別の鍵素材に交換できない。更新時は新しい KMS Key を追加し、Lambda が参照する Key ARN を切り替える。

Terraform への入力には、Key ID / ARN、GitHub App の Client ID、Owner 名と不変 Owner ID、Owner の種別（Organization またはユーザーアカウント）、GATES 用 Audience などの非秘密値を使用できます。GitHub App の Private Key は一切渡しません。

## 5. Lambda Deployment and Operation

Go バイナリは Linux の `arm64` 向けにビルドし、Lambda の `provided.al2023` 用 ZIP に `bootstrap` として配置します。Terraform はデプロイパッケージとそのハッシュを使って関数の更新を管理します。

Lambda のアーキテクチャを `arm64` とするのは、Go のバイナリを CGO を使わない静的バイナリとしてビルドするため、ネイティブ依存による制約がないからです。
同じメモリ量であれば、`arm64` は `x86_64` より実行時間あたりの単価が低くなります。

Lambda と KMS Key は単一のリージョンに置きます。
リージョンは Terraform の入力で受け取り、既定値を `ap-northeast-1` とします。
Composite Action に渡す Variables の `GATES_AWS_REGION` にも同じ値を設定します（[GitHub Actions Design](github-actions.md#21-inputs-and-outputs)）。

Lambda の設定では次を扱います。

| 設定 | 設計方針 |
|---|---|
| ネットワーク | 利用者管理 VPC に接続せず、GitHub API と OIDC JWKS への HTTPS 通信を確保 |
| タイムアウト・メモリ | JWT 検証と GitHub API / KMS の外部呼び出しに必要な処理時間・メモリを確保 |
| 同時実行数 | Reserved Concurrency による実行数の上限を設定可能とする |
| 環境変数 | GitHub App の Client ID、Owner 名と不変 Owner ID、Owner の種別、GATES 用 Audience、KMS Key ARN などの非秘密設定。実行時の要求からは変更できない |
| 監査ログ | ログ保持期間、アクセス権限、相関 ID、非秘密の認証・発行結果。保持期間は Terraform の入力で受け取り、既定値を7日とする |

ログの保持期間に求められる長さは、組織の監査方針によって異なります。
そのため GATES は保持期間の値を強制せず、ログを保存し、保持期間を設定できるようにするところまでを責務とします。
運用者は、自組織の方針に従って保持期間の入力を変更します。

## 6. Traceability

| 要求 | AWS における実現手段 |
|---|---|
| FR-001、FR-005、FR-025、AC-25、AC-26 | IAM OIDC Provider、不変 ID 入り `sub` を条件とする IAM Role Trust Policy、対象 Lambda に限定した Invoke 権限、IAM 認証付き Invoke 以外の呼び出し口を設けない Lambda 構成 |
| FR-013、QR-001、QR-006、AC-20 | `EXTERNAL` origin の非対称署名 KMS Key、Lambda 実行 Role への `kms:Sign` のみの許可、Key Policy による署名権限と管理権限の分離 |
| FR-017、FR-023、AC-14 | 新 KMS Key の追加による鍵更新、`prevent_destroy` と Plan の確認による置換・削除の防止 |
| FR-022、QR-006、AC-21 | AWS リソースの Terraform 管理と、鍵素材インポートおよびインポート前コピー削除の分離 |
| FR-024、AC-24 | 許可 Owner、GitHub App、KMS Key を Terraform 入力と Lambda 環境変数で固定。Installation は固定した GitHub App と不変 Owner ID から一意に導出する（[Security Design](security.md#32-installation-scope-as-authorization-boundary)） |
| FR-021、QR-004 | CloudWatch Logs の保持期間・アクセス制限と資格情報の非記録 |

受け入れ条件の正本は [Requirements](../spec/requirements.md) です。

## 7. Sources

- [GitHub — OpenID Connect reference](https://docs.github.com/en/actions/reference/security/oidc)
- [AWS — IAM and AWS STS condition context keys](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_iam-condition-keys.html)
- [AWS — Creating KMS keys with imported key material](https://docs.aws.amazon.com/kms/latest/developerguide/importing-keys-create-cmk.html)
- [AWS — Requirements for imported key material](https://docs.aws.amazon.com/kms/latest/developerguide/importing-keys-conceptual.html)
- [AWS — Importing key material](https://docs.aws.amazon.com/kms/latest/developerguide/importing-keys-import-key-material.html)
- [AWS — Rotate AWS KMS keys](https://docs.aws.amazon.com/kms/latest/developerguide/rotate-keys.html)
- [AWS — KMS key policies](https://docs.aws.amazon.com/kms/latest/developerguide/key-policies.html)
- [AWS — KMS Sign](https://docs.aws.amazon.com/kms/latest/APIReference/API_Sign.html)
- [AWS — Lambda Go deployment](https://docs.aws.amazon.com/lambda/latest/dg/lambda-golang.html)
- [AWS — Lambda network troubleshooting](https://docs.aws.amazon.com/lambda/latest/dg/troubleshooting-networking.html)
- [Terraform — AWS KMS External Key](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/kms_external_key)
- [Terraform AWS Provider — CHANGELOG（v6.11.0）](https://github.com/hashicorp/terraform-provider-aws/blob/main/CHANGELOG.md)
- [Terraform — Managing sensitive data](https://developer.hashicorp.com/terraform/language/manage-sensitive-data)
- [Terraform — Lifecycle](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)
