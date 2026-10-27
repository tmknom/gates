# Security

## 1. Security Model

本システムでは、AWS IAM による Lambda Invoke の入口制御と、Lambda 内で検証した GitHub OIDC JWT に基づくトークン発行認可を分離します。GitHub App Private Key は AWS KMS に置き、Lambda は署名操作を KMS に依頼します。発行する Installation Access Token は、要求されたリポジトリと `contents:read` に限定します。

コンポーネントの全体構成、JWT の生成元、正常系の処理順序は [System Architecture](architecture.md) に記述します。本書は各境界が保証する内容と、認可・資格情報保護・拒否条件を定義します。

## 2. Trust Boundaries

| 境界 | 保証すること | 保証しないこと |
|---|---|---|
| AWS IAM / STS | 許可 Organization の不変 Owner ID を `sub` に含む GitHub OIDC JWT からだけ AWS 一時認証情報を発行し、特定 Lambda の Invoke のみ許可する | ワークフロー・リポジトリ単位のトークン発行認可、資格情報の現在の利用者の特定 |
| OIDC Verifier | GitHub JWT の真正性、Issuer、Audience、期限、Organization / Repository の不変 ID を検証する | JWT と AWS 一時認証情報が同じジョブで取得されたことの証明、トークン発行権限そのもの |
| Authorization | 許可 Owner ID、取得した Installation の `account.id`、固定権限、明示された要求を照合する | アクセス先ごとの Caller の同意確認、対象リポジトリが Installation に含まれることの事前確認 |
| AWS KMS | 非抽出可能な秘密鍵による RSA 署名を実行する | 署名対象 JWT の内容に対する GATES の認可判断 |
| GitHub API | Installation と App の権限上限に従い短命トークンを発行する | GATES 側のトークン発行認可 |

### 2.1 AWS IAM / STS

IAM Role の Trust Policy は、GitHub OIDC Provider を信頼先とし、`aud` と `sub` の条件で引き受けを制限します。AWS IAM は `repository_owner_id` のような GitHub 固有の Claim も条件キーとして評価できますが、この Claim は名前だけの旧形式の `sub` を持つトークンにも含まれるため、旧形式の拒否には使えません。そのため不変 Owner ID の制約と旧形式の拒否は、不変 ID 入りの `sub`（`repo:OWNER@OWNER-ID/REPO@REPO-ID:...`）に対するパターン条件として表現します。具体的な条件式は [AWS Infrastructure Design](aws-infrastructure.md#31-github-actions-iam-role) に記述します。

- `aud` を `sts.amazonaws.com` に固定する。
- `sub` は、許可 Organization の名前と不変 Owner ID の組で始まる不変 ID 入りの形式だけを受け付け、名前だけの旧形式を拒否する。Organization 内での共通利用を前提とし、`ref`、`pull_request`、`environment` などの実行文脈による追加制限は設けない。
- 引き受けた IAM Role の権限は対象 Lambda への `lambda:InvokeFunction` のみとし、KMS `Sign`、IAM 管理、鍵インポートを許可しない。
- 許可 Organization のワークフローは共通の一つの IAM Role を引き受ける。

Organization 側で既存リポジトリの OIDC トークンに不変 ID を含める設定が済んでいることを前提とします。この設定は GitHub 側の操作であり、GATES は実施しません（[Requirements](../spec/requirements.md) §2.2）。設定前のリポジトリは旧形式の `sub` を持つため、IAM Role を引き受けられません。

IAM は認証情報の由来となる Organization に対する入口制限であるため、IAM 認証情報の窃取・再利用により、元のジョブ以外から Lambda Invoke が行われる可能性は残ります。

### 2.2 GATES 用 OIDC JWT

STS 用 JWT とは別に、GATES 用 Audience の OIDC JWT を取得します。Lambda は固定された GitHub Issuer の Discovery / JWKS を利用し、許容アルゴリズム、署名、Issuer、Audience、有効期限、必須 Claim を検証します。Caller は `repository_owner_id` と `repository_id` の Claim から確定し、`sub` が不変 ID 入りの形式で、含まれる ID がこれらの Claim と一致することも確認します。外部から指定された任意の Issuer / JWKS URL を使用しません。

AWS 一時認証情報と GATES 用 OIDC JWT が同じジョブで取得されたことを、暗号学的に検証しません。AWS IAM による Invoke の許可と、GATES 用 OIDC JWT に基づく発行認可は独立した境界です。トークンの発行可否は GATES 用 OIDC JWT と固定された許可条件から判断し、Composite Action 側の自己申告による照合を認可根拠としません。

## 3. Authorization

### 3.1 Authorization Rule

リポジトリへの発行は、次の条件がすべて成立した場合だけ許可します。

1. 検証済み Caller の `repository_owner_id` が許可 Organization の不変 Owner ID と一致する。Organization 名の一致では代用しない。
2. 要求した各 `owner/repo` の Owner 名が固定された許可 Organization 名と一致する。
3. GitHub App JWT で取得した Organization Installation の `account.id` が許可 Owner ID と一致する。
4. 要求が `repository` スコープであり、Permission Map が **`contents:read` のみ** である。

Caller の `repository_id` は監査のための識別に用い、認可条件には含めません。

条件 4 の Scope と Permission Map は、形式が正しければ JWT 検証後に認可条件として判定し、許可外なら認可拒否として扱います。入力検証で先に落とさないのは、`contents:write` のような許可外の要求を、認証済み Caller とともに監査記録に残すためです。

これらの条件を確認した後、Installation ID、要求したリポジトリ名、権限を含む `TokenGrant` を生成します。Installation に対象リポジトリが含まれることと、発行結果が Grant に一致することは、GitHub API の応答を受けてから確認します。一部だけで発行することや権限を暗黙に広げることは禁止します。

### 3.2 Installation Scope as Authorization Boundary

許可された Organization の有効なワークフローは、GATES で利用する GitHub App がインストールされたリポジトリへの `contents:read` トークンを要求できます。アクセス先のチームが GATES を利用するかどうかには依存しません。

GitHub App のインストール対象が読み取り可能範囲の上限となります。`Only select repositories` により対象を限定できます。`All repositories` を使用すると、その Organization にインストールされた App の対象リポジトリ全体が読み取り可能範囲に含まれます。インストール対象の変更は認可境界の変更として扱います。

GitHub App は一つの Organization に一度しかインストールできないため、デプロイ時に固定した GitHub App と許可 Organization の組から Installation は一意に決まります。Lambda は Installation ID を設定として持たず、固定した Organization の Installation を GitHub API から取得し、`account.id` の一致で確認します。この導出は [Requirements](../spec/requirements.md) FR-024 に従います。Installation ID を設定値として持たないため、アプリの再インストールで ID が変わっても設定変更は不要であり、実行時の入力から Installation を指定する余地もありません。Organization の名前変更やアプリの再インストールが起きても、固定した不変 Owner ID と異なる Installation へ発行しません。

Organization の名前変更時に Trust Policy と Lambda の設定を更新するまで発行が止まる扱いは、[AWS Infrastructure Design](aws-infrastructure.md#31-github-actions-iam-role) に記述します。

### 3.3 Permission Boundary

Lambda が許可する権限は `contents:read` に固定します。要求された Scope や Permission が許可条件と異なる場合は拒否します。GitHub App の権限上限が GATES の許可範囲より広くても、発行するトークンの権限は拡大しません。

### 3.4 Issuance Result Verification

- GitHub API に発行を要求するときは、認可された `TokenGrant` の `repositories` と `permissions` を必ず指定する。
- 発行応答の対象リポジトリと実効権限を `TokenGrant` に照合する。Owner ID、Repository ID の有効性・一意性、対象の全件一致を確認し、必要な情報の欠落、対象外のリポジトリ、過大な権限があればトークンを返却しない。この拒否は GitHub API の通信障害とは別のエラーとして返し、監査記録に Caller とともに残す。
- 要求にかかわらず GitHub が付与する `metadata:read` だけは、応答に含まれていても許容する。
- 応答の Repository ID は発行結果の確認・監査に利用し、要求時に事前検証した ID と同一であることを証明するものではない。

GitHub API の要求・応答形式と具体的な照合条件は [Lambda Design](lambda.md) に記述します。

## 4. Secret and Token Lifecycle

KMS Key の構成・IAM 権限は Terraform で管理します。GitHub App Private Key の取得・変換・鍵素材インポートは独立した運用で行い、Terraform の State / Plan と Go アプリケーションには鍵素材を渡しません。

| 資格情報 | 保存・利用の境界 |
|---|---|
| GitHub App Private Key | KMS インポート前は運用者が別の運用境界で扱う。Lambda と Runner には渡さない。インポート後、インポート前のコピーは運用者の手元から削除する。KMS にインポートした鍵素材は取り出せないため、KMS Key を失った場合は GitHub 側で新しい秘密鍵を発行して切り替える |
| AWS KMS Key | インポート済みの非対称署名鍵。Lambda 実行 Role に当該キーの `kms:Sign` のみ許可。鍵更新時は新 KMS Key を用意 |
| GitHub App JWT | KMS 署名で Lambda が短時間生成し、GitHub API 呼び出しにのみ使用。外部返却・ログ出力禁止 |
| Installation Token | 要求・認可された対象と `contents:read` に限定して Runner へ返す。短命でも窃取リスクは残る |
| AWS 一時認証情報・GATES 用 OIDC JWT | Runner で短時間だけ保持し、ログやデバッグ出力に含めない |

署名アルゴリズムと JWT の組み立ては [Lambda Design](lambda.md#72-kms-signing) に記述します。

## 5. Failure, Audit, and Security Risks

- 入力が不正、または JWT の検証に失敗したら、GitHub API や KMS のトークン発行処理に進まない。対象解決や署名の失敗、発行結果の不一致は Fail Closed とする。
- 監査ログには相関 ID、時刻、処理結果（成功またはエラーコード）に加え、認証済み Caller の Owner ID と Repository ID、要求対象（`owner/repo`）、要求権限を記録する。認証前の失敗では Caller が確定していないため、未検証の値を認証済み主体として記録しない。秘密鍵、OIDC JWT、GitHub App JWT、Installation Token、AWS 一時認証情報は記録しない。
- AWS Invoke の成功と Lambda の業務成功を区別し、Composite Action は `FunctionError` と応答の両方を確認する。
- Lambda の実行ロールが侵害された場合、KMS `Sign` による不正署名が可能となる。KMS は秘密鍵を抽出不可にするが、署名能力の悪用を防ぐものではない。
- Organization への所属はワークフロー内のコードの信頼性を保証しない。`pull_request_target` などで信頼できない Pull Request のコードを実行すると、正当な JWT と AWS 認証情報を取得できるジョブの権限を悪用され得る。利用者は資格情報を取得するジョブで信頼できないコードを実行しない。
- JWT、AWS 認証情報、Installation Token の漏えい・再利用を完全には防止しない。許可範囲、期限、権限とインストール対象の限定で影響を抑える。

## 6. Traceability

| 機能 | 対応要求 |
|---|---|
| 入口制御と Caller の識別（§2） | FR-001〜005、FR-025、QR-007 |
| 発行認可（§3.1〜3.3） | FR-006〜012、FR-024、QR-002、QR-007 |
| 署名と発行結果の検証（§3.4、§4） | FR-013〜018、QR-001、QR-003、QR-006 |
| 失敗時の扱いと監査（§5） | FR-020、FR-021、QR-003、QR-004 |

受け入れ条件は [Requirements](../spec/requirements.md) を正本とします。

## 7. Sources

- [GitHub — OpenID Connect reference](https://docs.github.com/en/actions/reference/security/oidc)
- [GitHub — Securely using pull_request_target](https://docs.github.com/en/actions/reference/security/securely-using-pull_request_target)
- [GitHub — Generating an installation access token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app)
- [GitHub — REST API endpoints for GitHub Apps](https://docs.github.com/en/rest/apps/apps)
- [AWS — IAM and AWS STS condition context keys](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_iam-condition-keys.html)
- [AWS — Lambda Invoke API](https://docs.aws.amazon.com/lambda/latest/api/API_Invoke.html)
- [AWS — KMS Sign API](https://docs.aws.amazon.com/kms/latest/APIReference/API_Sign.html)
- [AWS — Imported key material](https://docs.aws.amazon.com/kms/latest/developerguide/importing-keys-import-key-material.html)
- [AWS — KMS key policies](https://docs.aws.amazon.com/kms/latest/developerguide/key-policies.html)
