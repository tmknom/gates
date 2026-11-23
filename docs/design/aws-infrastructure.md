# AWS Infrastructure Design

## 1. Scope and Responsibility

AWS のリソースは、IAM OIDC Provider を除いて Terraform で管理します。
AWS 側が受け持つのは、GitHub OIDC から AWS STS を経た IAM 認証付きの Lambda の直接 Invoke と、KMS `Sign` による署名です。
リポジトリ単位の本人確認とトークンの発行認可は Lambda の中で行い、各境界が保証する範囲は [Security Design](security.md) に定めます。

GitHub App Private Key の取得、変換、KMS への鍵素材のインポートは Terraform の対象外とし、独立した手順で行います。
鍵素材を読み込む Terraform の Resource、Data Source、Provisioner、外部実行処理は作りません。
Terraform への入力は、Key ID / ARN、GitHub App の Client ID、許可オーナー、GATES 用 Audience、発行上限、保護対象の一覧などの非秘密値に限ります。

## 2. Resource Model

| 対象 | 責務 | 設計上の制約 |
|---|---|---|
| IAM OIDC Provider | `https://token.actions.githubusercontent.com` を AWS STS の信頼先にする | Terraform の管理外で用意し、参照する。この URL の Provider はアカウントで一つだけの共有リソースで、GATES の State に入れると、GATES の削除が他の用途の OIDC 連携を壊すため。Client ID に `sts.amazonaws.com` を含める |
| GitHub Actions IAM Role | 許可オーナーのワークフローに `AssumeRoleWithWebIdentity` を許可する | Trust Policy は §3.1。Permissions Policy は GATES 用の Lambda への `lambda:InvokeFunction` だけ |
| Lambda 実行 IAM Role | Lambda から KMS と CloudWatch Logs を呼び出す | 使用中の鍵への `kms:Sign` と、ログの書き込みだけ（§3.2） |
| Permissions Boundary | 2 つの IAM Role の権限の上限を示す | GATES の Terraform が管理する（§3.3） |
| Lambda Function | JWT 検証、発行認可、KMS への署名依頼、GitHub API 通信 | IAM 認証付きの Invoke API からだけ呼び出す。Function URL、API Gateway などの別の呼び出し口を設けず、他アカウントや匿名主体を許可するリソースベースポリシーを付けない（§5） |
| KMS Key | GitHub App Private Key の鍵素材をインポートした署名鍵 | 鍵の更新の間は新旧 2 つを置く（§4） |
| CloudWatch Logs | Lambda のログと監査記録を保管する | Log Group を Terraform が作り、保持期間を設定する |

GitHub App の作成とインストール対象は GitHub 側で管理します。

## 3. IAM Design

### 3.1 GitHub Actions IAM Role

Trust Policy は、Security Design の入口制御を、`aud` と `sub` の条件で表します。
`aud` は `StringEquals` で `sts.amazonaws.com` と一致させ、`sub` は `StringLike` で `repo:<OWNER-NAME>@<OWNER-ID>/*` と一致させます。
不変 ID 入りの `sub` は `repo:OWNER@OWNER-ID/REPO@REPO-ID:<ref や environment など>` の形式です（[GitHub Actions OIDC Constraints](../external/github/oidc.md)）。

不変 Owner ID の制約と旧形式の拒否は、`repository_owner_id` などの GitHub 固有の Claim の条件キーでは表せません。
これらの Claim は、名前だけの旧形式の `sub` を持つ OIDC JWT にも含まれるためです。
そこで `sub` のパターンの先頭を、オーナー名と不変 Owner ID の組 `repo:<OWNER-NAME>@<OWNER-ID>/` に固定します。
GitHub のユーザー名とリポジトリ名には `@` を使えないため、旧形式の `repo:<OWNER-NAME>/<REPO>:...` がこの先頭に一致することはありません。

Owner ID の前を `*` にした `repo:*@<OWNER-ID>/*` では、他のオーナーを拒否できません。
`StringLike` の `*` は `/` や `:` を含む任意の文字列に一致するため、他のオーナーのワークフローがブランチ名に `@<OWNER-ID>/` を含めるだけで一致するからです。

オーナー名を条件に含めるため、オーナーの名前を変えると、Trust Policy と Lambda の環境変数 `GATES_ALLOWED_OWNER_NAME` を更新するまで引き受けが拒否されます。
この拒否は Fail Closed の動作として許容します。

### 3.2 Lambda Execution Role and Key Policy

署名の可否を決めるのは KMS Key Policy です。
Key Policy は実行 Role を Principal として直接許可し、`kms:Sign` だけを、署名アルゴリズムを `RSASSA_PKCS1_V1_5_SHA_256` に限って許します。
そのため署名の成立に実行 Role の IAM ポリシーは要らず、IAM ポリシーは Role の権限を IAM からも確認できるように、使用中の鍵への `kms:Sign` を明示するものです。
一方、Permissions Boundary は Key Policy が Role を直接許可していても効くため、Boundary には `kms:Sign` の許可が要ります。

鍵の管理操作は、アカウントの root を Principal にして、操作を列挙して許可します。
root の Principal は IAM ポリシーへの委任を意味するため、特定の Role を失っても鍵を管理し続けられます。
列挙に `kms:Sign` を含めないため、IAM ポリシーだけでは GATES の署名鍵で署名できません。
列挙する操作の範囲と、この設定で抑えるリスクと受容するリスクは [Security Design](security.md#51-kms-signing-key-risks) に定めます。

実行 Role には、鍵の管理操作と Key Policy の変更権限を付けず、Trust Policy の信頼先を Lambda サービスに限ります。
CloudWatch Logs の権限は、Terraform が作る Log Group へのログストリームの作成と書き込みに限り、`logs:CreateLogGroup` を付けません。
Log Group を消したときに Lambda が保持期間なしで作り直し、保持期間の設定が失われるのを防ぐためです。

### 3.3 Permissions Boundary

2 つの IAM Role には、GATES の Terraform が作る Permissions Boundary を付け、Role の権限に必要な操作だけを許可します。
GATES が Boundary を管理するのは、Role の権限に必要な操作を GATES が知っており、Role のポリシーと Boundary を同じ変更で更新できるためです。
Boundary は Role のポリシーと同じ適用主体が管理するため、適用主体に対する独立した歯止めにはなりません。
Boundary が抑えるのは、Terraform の管理外で Role に追加されたポリシーのうち、上限を超える権限です。

Boundary の Resource は、Lambda と KMS Key の ARN を参照しません。
Key Policy が実行 Role を、Lambda が実行 Role を、実行 Role が Boundary を参照するため、Boundary がそれらを参照すると Terraform の依存が循環するからです。
Lambda は名前から組み立てた ARN で指します。
KMS Key は、Key を参照しない Condition（`kms:KeyOrigin` が `EXTERNAL`、`kms:SigningAlgorithm` が `RSASSA_PKCS1_V1_5_SHA_256`）で、同じアカウントとリージョンの鍵素材をインポートした鍵に狭めます。
1 つの Boundary を 2 つの Role で共有するため、GitHub Actions IAM Role の上限にも `kms:Sign` が含まれます。
この範囲で残るリスクは [Security Design](security.md#51-kms-signing-key-risks) に定めます。

Boundary の名前は `gates-permissions-boundary` に固定します。
Role の作成に特定の Boundary を求めるアカウントが、GATES を適用する前に、この名前から組み立てた ARN を許可の対象に加えられるようにするためです。
アカウント側がデータソースで Boundary を参照すると、Boundary の作成より先にアカウント側を適用できず、GATES の適用との順序が循環します。
許可の対象に加えたアカウントは、Boundary の内容を GATES の適用主体に委ねることになり、GATES を適用するまでは、別の主体が同じ名前のポリシーを任意の内容で作れます。

### 3.4 Terraform Apply and State

Terraform を適用する主体には、リソース作成、ポリシー更新、コード配布に必要な権限を与え、KMS 鍵素材のインポートに必要な権限は通常の適用から分離します。
適用主体に強い管理権限が要る点は、実行主体の限定と変更レビューで制御します。

Terraform は運用者の手元から適用し、CI からは Plan も Apply も実行しません。
CI に適用権限を与えると、IAM と KMS Key Policy を変更できる強い権限を CI に置くことになるためです。

State は、適用する運用者の手元に置きます（ローカルバックエンド）。
State に秘密鍵素材は入りませんが、IAM と Lambda の構成が入るため、Git に含めず、静的解析のコンテナからも読めない場所に置きます。

## 4. KMS Key

### 4.1 Key Resource

KMS Key は、鍵素材を持たない `EXTERNAL` origin の非対称署名鍵（`key_usage=SIGN_VERIFY`）として作ります。
鍵素材の初回インポートまでは `PendingImport` で、署名できません。
`key_spec` は、RSA_2048 / RSA_3072 / RSA_4096 のうち GitHub App Private Key の鍵長と一致する値にします。
GitHub が生成する鍵の長さは確定していないため（[GitHub App Constraints](../external/github/app.md)）、インポートする鍵ごとに確かめます。

Terraform では、`hashicorp/aws` Provider の `aws_kms_external_key` で KMS Key と Key Policy を管理します。
`aws_kms_external_key` が非対称鍵の `key_spec` を受け付けるのは Provider v6.11.0 以降です。
鍵素材のインポートと、鍵の有効と無効の状態は Terraform で扱わず、次の属性を設定に書きません。

- **`key_material_base64`**：256 bit の対称鍵素材を Terraform からインポートする引数で、RSA の鍵素材には使えず、値が State に平文で保存される
- **`enabled`**：設定で省略すると AWS 側の値を State に取り込む属性のため、手動のインポートで鍵の状態が `PendingImport` から `Enabled` に変わっても、Plan に差分が出ない
- **`valid_to`**：手動のインポートでは鍵素材を無期限（`KEY_MATERIAL_DOES_NOT_EXPIRE`）にする。有効期限を付けると、Terraform は `valid_to` の差分を鍵素材の再インポートで解消しようとするが、鍵素材を設定に持たないため、その更新は成り立たない

Lambda は KMS Key を Key ARN の環境変数で参照し、KMS Alias を使いません。
Alias で参照すると、Alias を更新できる主体が Lambda の設定を変えずに署名鍵を差し替えられ、Key Policy とは別に守る対象が増えるためです。

### 4.2 Key Set and Rotation

インポートした非対称 KMS Key の鍵素材は別の鍵素材に交換できず、非対称鍵は KMS の自動ローテーションとオンデマンドローテーションにも対応しません。
そのため鍵の更新は、新しい KMS Key に鍵素材をインポートしてから Lambda の参照を切り替えて行い、新旧 2 つの KMS Key が同時にある期間が生じます。

Terraform では、KMS Key を名前の付いた集合（**鍵の集合**）として持ち、そのうち 1 つを **使用中の鍵** として指します。
使用中の鍵は、Lambda が署名に使う KMS Key です。
KMS の鍵の状態の有効（`Enabled`）と無効とは別の概念で、使用中でない鍵も `Enabled` であり得ます。

- 鍵の集合に置ける鍵は 1 つか 2 つに限る。どの鍵の Key Policy も実行 Role に `kms:Sign` を許可するため、実行 Role が署名できる鍵を、切り替えの前後の 2 つまでにする
- Lambda の環境変数 `GATES_KMS_KEY_ARN` と、実行 Role の IAM ポリシーは、使用中の鍵だけを参照する。切り替えは、この 2 つの更新になる
- どの鍵にも同じ Key Policy を付け、使用中かどうかで Key Policy を変えない。その理由と、実行 Role が 2 つの鍵で署名できる期間のリスクは [Security Design](security.md#51-kms-signing-key-risks) に定める

### 4.3 Key Deletion and Retirement

既存の KMS Key を Terraform の操作で置換・削除しないよう、`prevent_destroy` を設定します。
`prevent_destroy` は鍵の削除・置換を含む Plan をエラーで拒否し、鍵の集合から鍵を除いた Plan も拒否しますが、リソースの設定自体を取り除いた場合の削除は防ぎません。
Plan に鍵の削除が現れたら、適用を中止して確かめます。

集合から除く鍵（**退役させる鍵**）は、鍵の集合から除くのと同時に、削除せずに State から外して Terraform の管理を終えます。
Terraform に削除させないのは、`prevent_destroy` を外すと、同じリソースの使用中の鍵も守られなくなるためです。
State から外す操作は `terraform state rm` で行い、対象を退役させる鍵のアドレスに限った `make` のターゲットから実行します。

State から外した鍵の削除の予約（`kms:ScheduleKeyDeletion`）は、GitHub 側で旧鍵を削除して旧鍵への切り戻しがあり得なくなった後に、運用者が KMS の API で行います。
予約期間は、KMS が許す最短の 7 日にします。
GitHub 側で削除した後の旧鍵は、署名できても GitHub App として認証に使えない見込みで、残す理由がないためです。

## 5. Lambda Deployment and Operation

Go バイナリは、CGO を使わない静的バイナリとして Linux の `arm64` 向けにビルドし、Lambda の `provided.al2023` 用 ZIP に `bootstrap` として配置します。
`arm64` にするのは、静的バイナリにはネイティブ依存による制約がなく、同じメモリ量であれば `x86_64` より実行時間あたりの単価が低いためです。
Lambda と KMS Key は、Terraform の入力で受け取る単一のリージョンに置きます。

| 設定 | 設計方針 |
|---|---|
| ネットワーク | 利用者管理 VPC に接続せず、GitHub API と OIDC JWKS への HTTPS 通信を確保する |
| タイムアウト・メモリ | タイムアウトは、外部呼び出しの待ち時間の上限の合計より長くする |
| 同時実行数 | Reserved Concurrency による実行数の上限を設定できるようにする |
| 環境変数 | 非秘密の固定設定。実行時の要求からは変更できない |
| ログの保持期間 | Terraform の入力で受け取る |

ログの保持期間に求められる長さは、組織の監査方針によって異なります。
そのため GATES は保持期間の値を強制せず、運用者が自組織の方針に合わせて入力を変更します。

### 5.1 Environment Variables

環境変数の名前は、Lambda と Terraform の間の契約として次のとおり定めます。

| 環境変数 | 値 |
|---|---|
| `GATES_ALLOWED_OWNER_ID` | 不変 Owner ID。符号と先頭の `0` を付けない 10 進表記 |
| `GATES_ALLOWED_OWNER_NAME` | オーナー名 |
| `GATES_ALLOWED_OWNER_TYPE` | オーナーの種別（`Organization` または `User`） |
| `GATES_GITHUB_APP_CLIENT_ID` | GitHub App の Client ID |
| `GATES_AUDIENCE` | GATES 用 Audience |
| `GATES_KMS_KEY_ARN` | 使用中の鍵の KMS Key ARN |
| `GATES_ISSUABLE_PERMISSIONS` | 発行上限。権限名と水準を `:` でつないだ組をカンマで区切って並べる（`contents:read,pull_requests:write`） |
| `GATES_PROTECTED_REPOSITORIES` | 保護対象の一覧。YAML の文書 1 件。保護対象を置かないデプロイは `repositories: []` を書く |

Issuer と GitHub API Host は環境変数にしません。
デプロイごとに変わらない値であり、環境変数にすると、Lambda の設定を変更できる主体が検証の基準を差し替えられるためです。

発行上限と保護対象の一覧は、既定値のない Terraform の変数で受け取ります。
発行してよい権限と、保護対象を置くかどうかを、運用者に明示して決めさせるためです。
保護対象の一覧の既定値を空の一覧にすると、変数を書き忘れたデプロイが、保護のないまま Plan と Apply を通ります。
保護対象の一覧の値は、コメントを含めてそのまま環境変数に入ります。

### 5.2 Plan-Time Validation

Terraform の変数の検証は、環境変数の値のうち Lambda が初期化で拒否する値を、Plan の時点で拒否します。
例外は次のとおりで、どれも Terraform では確かめず、Lambda の初期化だけが拒否します。

- **許可権限に含まれない組**：発行上限と許可 Caller の権限の上限の組が許可権限に含まれること。Terraform で確かめるには、Go のコードが列挙する許可権限を Terraform にも写すことになり、列挙が 2 か所になって、一致を確かめる仕組みが別に要るためである
- **復号した値から見分けられない YAML の誤り**：保護対象の一覧は、Terraform の `yamldecode` で復号した値に対して確かめる。`yamldecode` はアンカーとエイリアスを展開し（[Terraform — yamldecode](https://developer.hashicorp.com/terraform/language/functions/yamldecode)）、重複したキーを後勝ちで読むなど、Lambda より多くの YAML を受け付けるため、重複したキー、アンカー、エイリアス、`yamldecode` が対応する型のタグ（`!!str` など）、ディレクティブ、BOM などの誤りは Plan を通る
- **文字列以外に解決される引用符のない名前と権限**：`yamldecode` は、YAML の Core Schema では文字列に解決される `yes` や `on` を真偽値に復号する。文字列であることを求めると、Lambda が受け付ける値まで拒否するため、名前と権限が文字列であることは確かめない

どの例外の誤りを含む設定も Plan と Apply を通り、適用した後の最初の Invoke で初期化の失敗になり、どの要求にもトークンを返しません。
Terraform の変数の説明に、Plan を通っても Lambda が拒否する誤りがあることを書きます。

逆に、Terraform の検証は、次の点で Lambda より狭い値だけを通します。

- Owner ID と、保護対象の一覧の Repository ID は、`int64` に必ず収まる 18 桁までに限る
- Client ID と Audience は、前後の空白を拒否する。前後の空白は書き損じであることが多く、Plan の時点で気付けるようにするためである
- Audience を `https://github.com` や `https://github.com/` で始まる値と比べるときは、Unicode の小文字化で比べる。Lambda は ASCII の範囲で比べる
- 保護対象の一覧の引用符のない名前は、`yamldecode` が復号した値の表記で、文字種と重複を確かめる。`2001-12-14` は日時の表記に変わって名前の文字種を外れ、`yes` と `on` はどちらも `true` になって重複と判定される。Lambda は書いたとおりの文字列として受け付けるが、Plan で拒否される。名前を引用符で囲めば、書いたとおりの文字列で確かめる

Lambda は、環境変数の合計が 4096 バイトを超える設定を Apply の時点で拒否します（[AWS Lambda Environment Size](../research/aws/lambda-environment-size.md)）。
保護対象の一覧は保護対象と許可 Caller の数に比例して長くなり、コメントも合計に数えられるため、Plan の時点で合計を確かめます。
数え方は AWS の資料のとおり、キー名、値、引用符、カンマ、括弧を含めた JSON の UTF-8 のバイト数です。
値の改行と二重引用符をエスケープの後の長さで数えるかは資料にないため、多めに見積もる側（エスケープの後）で数えます。
Plan の時点で値の決まらない KMS Key ARN は、ARN の形式から決まる長さで数えます。

## 6. Traceability

| 要求 | AWS における実現手段 |
|---|---|
| FR-AUTHN-003、FR-OPS-002、AC-OPS-005、AC-OPS-007、AC-AUTHN-003 | IAM OIDC Provider、不変 ID 入り `sub` を条件とする IAM Role Trust Policy、対象 Lambda に限定した Invoke 権限、IAM 認証付き Invoke 以外の呼び出し口を設けない Lambda 構成 |
| FR-ISSUANCE-001、QR-008、AC-OPS-005 | `EXTERNAL` origin の非対称署名 KMS Key、Lambda 実行 Role への `kms:Sign` のみの許可、Key Policy による署名権限と管理権限の分離 |
| FR-OPS-005、AC-ISSUANCE-005 | 鍵の集合への新 KMS Key の追加と使用中の鍵の切り替えによる鍵更新、`prevent_destroy` と Plan の確認による置換・削除の防止、退役させる鍵の State からの除外 |
| QR-008、AC-OPS-006 | 鍵素材インポートおよびインポート前コピー削除の Terraform からの分離 |
| FR-OPS-001、FR-AUTHZ-008、FR-AUTHZ-009、AC-AUTHZ-007、AC-AUTHZ-010、AC-AUTHZ-013 | 許可オーナー、GitHub App、KMS Key、発行上限、保護対象の一覧を Terraform 入力と Lambda 環境変数で固定。Plan の時点の変数の検証と、Lambda の初期化での残りの確認。Installation は固定した GitHub App と不変 Owner ID から一意に導出する |
| FR-OPS-003、QR-004 | CloudWatch Logs の保持期間・アクセス制限と資格情報の非記録 |

受け入れ条件の正本は [Acceptance Criteria](../spec/acceptance-criteria.md) です。

## 7. References

- [GitHub Actions OIDC Constraints](../external/github/oidc.md)
- [GitHub App Constraints](../external/github/app.md)
- [AWS Lambda Environment Size](../research/aws/lambda-environment-size.md)
- [AWS — IAM and AWS STS condition context keys](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_iam-condition-keys.html)
- [AWS — Create an AWS KMS key without key material](https://docs.aws.amazon.com/kms/latest/developerguide/importing-keys-create-cmk.html)
- [AWS — Create a KMS key with imported key material](https://docs.aws.amazon.com/kms/latest/developerguide/importing-keys-conceptual.html)
- [AWS — Import the key material](https://docs.aws.amazon.com/kms/latest/developerguide/importing-keys-import-key-material.html)
- [AWS — Rotate AWS KMS keys](https://docs.aws.amazon.com/kms/latest/developerguide/rotate-keys.html)
- [AWS — Deleting AWS KMS keys](https://docs.aws.amazon.com/kms/latest/developerguide/deleting-keys.html)
- [AWS — KMS key policies](https://docs.aws.amazon.com/kms/latest/developerguide/key-policies.html)
- [AWS — KMS condition keys](https://docs.aws.amazon.com/kms/latest/developerguide/conditions-kms.html)
- [AWS — KMS Sign](https://docs.aws.amazon.com/kms/latest/APIReference/API_Sign.html)
- [AWS — Lambda Go deployment](https://docs.aws.amazon.com/lambda/latest/dg/lambda-golang.html)
- [AWS — Lambda network troubleshooting](https://docs.aws.amazon.com/lambda/latest/dg/troubleshooting-networking.html)
- [AWS — Lambda quotas](https://docs.aws.amazon.com/lambda/latest/dg/gettingstarted-limits.html)
- [Terraform — AWS KMS External Key](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/kms_external_key)
- [Terraform AWS Provider — CHANGELOG（v6.11.0）](https://github.com/hashicorp/terraform-provider-aws/blob/main/CHANGELOG.md)
- [Terraform — Managing sensitive data](https://developer.hashicorp.com/terraform/language/manage-sensitive-data)
- [Terraform — yamldecode](https://developer.hashicorp.com/terraform/language/functions/yamldecode)
- [Terraform — Lifecycle](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)
- [Terraform — Remove a resource from state](https://developer.hashicorp.com/terraform/language/state/remove)
