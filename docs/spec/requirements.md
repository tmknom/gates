# Requirements

GATES は、GitHub Actions ワークフローに GitHub App Installation Access Token を発行するシステムです。

## 1. Purpose

現在のスコープ選択とその理由、後続段階で検討する能力は [Project Scope](project-scope.md) に記述します。本ドキュメントは、現在実現すべき要求と受け入れ条件を定めます。

GitHub Actions が `GITHUB_TOKEN` だけでは実現できないリポジトリ間の読み取りに対し、短命な GitHub App Installation Access Token を発行します。
GitHub App Private Key を GitHub Secrets や Actions Runner に置かず、署名を AWS KMS に委譲します。

許可された単一の **オーナー**（リポジトリを所有する GitHub の Organization またはユーザーアカウント）配下のワークフローは、GitHub App の **Installation**（GitHub App をオーナーにインストールした単位）に含まれるリポジトリに対して `contents:read` のトークンをセルフサービスで要求できます。
対象リポジトリと権限を利用者に明示させ、GitHub App に付与された権限全体の払い出しを防ぎます。

利用者向けの操作は Composite Action に集約し、ワークフローが GATES の認証・発行処理の詳細に依存しない構成とします。

## 2. Scope and Constraints

### 2.1 Technical Constraints

- Go、AWS Lambda、AWS KMS を使用する。
- GitHub App Private Key は GitHub が発行するため、AWS KMS に鍵素材としてインポートして使う。
- KMS にインポートした鍵素材は、KMS から平文で取得できない。ただし、インポート前の秘密鍵コピーは KMS によって自動的に消去されない。
- AWS リソースの管理に Terraform を使う。
- 現在のスコープで発行を許可する権限は `contents:read` に固定する。GitHub App / Installation に付与された権限が広くても、GATES の許可条件を超えて発行しない。GitHub が要求とは別に付与する権限の扱いは FR-018 に従う。スコープの判断理由は [Project Scope](project-scope.md) に置く。

### 2.2 Out of Scope

- 複数のオーナーの同時サポート
- 複数の GitHub App Installation の同時サポート
- 呼び出し元を名前だけで特定する形式の GitHub OIDC トークンを持つリポジトリのサポート。該当する要求は FR-005 により拒否する。
- OIDC トークンの `sub` のカスタマイズにより、実 JWT で確かめた形式から外れるリポジトリのサポート。受け付ける形式は [Lambda Design](../design/lambda.md#71-oidc-validation) に従う。
- GitHub Enterprise Server など GitHub.com 以外の GitHub への対応
- GitHub 側の設定と操作（GitHub App の旧 Private Key の失効、オーナー配下のリポジトリの OIDC トークンに不変 Owner ID を含める設定）。GATES はこれらを実施も代行もしない。
- OIDC JWT の有効期限内の再利用の検出。再利用できる期間は、FR-002 の時間条件だけで制限する。
- 連続した要求に対するレート制限と濫用の検出
- IAM 層で拒否された呼び出し（FR-025）の監査記録。記録されるかどうかは AWS の仕様に依存し、GATES は保証しない。

### 2.3 Current Access Boundary

現在のスコープでは、許可オーナーのワークフローは、同一の GitHub App Installation に含まれるリポジトリへの `contents:read` を要求できます。Caller 別・チーム別にアクセス先を限定する認可は設けません。対象と権限は要求に明示させ、GATES の許可条件と Installation の上限で制限します。

この境界に伴う横断アクセスの残留リスクと緩和策は [Security Design](../design/security.md#32-installation-scope-as-authorization-boundary) に記述します。細粒度認可を現在のスコープから外した判断理由と、発行可能権限を将来拡大する可能性は [Project Scope](project-scope.md) を正本とします。

## 3. Stakeholders and Responsibilities

| 主体 | 必要とすること | 責任 |
|---|---|---|
| ワークフロー利用者 | 必要なリポジトリと権限を指定し、運用者が用意した接続情報を参照するだけでトークンを取得できる | Composite Action を呼び出すワークフローで `permissions: id-token: write` を宣言する。Composite Action に対象リポジトリと権限、および GATES 運用者が提供する接続情報を渡す。取得したトークンを自身のワークフロー内で適切に扱う。 |
| GitHub App / オーナーの管理者 | GATES を通じて読み取られ得るリポジトリを Installation の対象で管理できる | GitHub App の Installation の対象を管理する。 |
| GATES 運用者 | 秘密鍵・認可・最小権限の発行を少ない構成要素で保守できる | 認証、認可、トークン発行、秘密鍵の保護、監査を担う。GATES の呼び出しに使う IAM ロールを引き受けられる主体の条件（信頼ポリシー）を管理する。Composite Action を提供し、そのインターフェースを維持する。Composite Action に渡す接続情報（IAM ロール、Lambda 関数、AWS リージョン、GATES 用 Audience）を、ワークフローから参照できるよう GitHub 側に用意する。 |

## 4. Use Cases

| Use Case | 成立条件 |
|---|---|
| Installation に含まれる一つまたは複数のリポジトリを読み取る | 許可された呼び出し元、同一オーナー / Installation、明示された `contents:read` |
| GitHub App Private Key を更新する | 新しい鍵に切り替える。GitHub 側の旧鍵失効は対象外（§2.2） |
| 認証不正・対象範囲外・過大な要求を拒否する | JWT、リポジトリ、権限の検証が成立しなければトークンを返さない |

## 5. Functional Requirements

### 5.1 Authentication

| ID | Requirement |
|---|---|
| FR-001 | 許可されたオーナーの GitHub Actions ワークフローからの呼び出しだけを受け付ける。 |
| FR-002 | GATES 用 Audience の GitHub OIDC JWT の署名・Issuer・Audience・時間条件・必須 Claim を検証し、不変 Owner ID / Repository ID で呼び出し元を識別する。これで特定される呼び出し元を **Caller** と呼ぶ。Repository ID は監査のための Caller 識別に用い、認可には用いない。ワークフロー実行を特定する `run_id` と `run_attempt`、およびワークフロー起動属性（起動したイベント、ref、ワークフロー、environment、行為者など）は、監査のためだけに Caller に添え、認可には用いない。ワークフロー起動属性は JWT に含まれる範囲で読み取り、欠落を拒否の理由にしない。 |
| FR-003 | 許可されない呼び出し元、JWT の欠落・改ざん・期限切れ・Audience 不一致・不正な主体・異なるオーナーを拒否する。 |
| FR-004 | 要求者の自己申告を Caller の根拠にしない。 |
| FR-005 | GATES の呼び出しに使う IAM ロールの引き受けでは、呼び出し元を不変 Owner ID で特定できる GitHub OIDC トークンだけを受け付け、名前だけで特定するトークンを拒否する。 |

### 5.2 Authorization

| ID | Requirement |
|---|---|
| FR-006 | 検証済み JWT の不変 Owner ID が、許可オーナーの不変 Owner ID と一致することを確認する。オーナー名の一致では代用しない。 |
| FR-007 | 許可オーナーに属し、GATES で利用する GitHub App の同一 Installation に含まれるリポジトリだけに発行する。含まれない対象を一つでも含む要求にはトークンを返さない。発行前に確認するか発行結果で検出するかは設計に委ねる。発行結果で検出し、トークン文字列を取得できた場合は失効を試みる。 |
| FR-008 | 現在のスコープでは `contents:read` のみ許可し、その他の権限を拒否する。対象は要求で明示したリポジトリ（GitHub の `repository_selection` でいう `selected`）に限り、全リポジトリ（`all`）を対象にする手段を利用者に与えない。 |
| FR-009 | 利用者の入力から Caller、Installation ID、認可の根拠や任意の API Host を指定できないようにする。 |
| FR-010 | 要求されたリポジトリ名のオーナーと、GitHub API が返す実効対象を検証し、要求外のリポジトリへ発行しない。 |
| FR-011 | 複数対象の要求は全件が成立する場合のみ許可し、一部成功や要求範囲の自動的な縮小を行わない。 |
| FR-012 | 利用者に対象リポジトリと権限の明示を要求し、欠落・未知値を広い既定権限に読み替えない。 |

### 5.3 Token Issuance

| ID | Requirement |
|---|---|
| FR-013 | 秘密鍵の平文を取得せずに GitHub App JWT を生成する。 |
| FR-014 | GitHub App Installation に付与された対象・権限を上限として、GitHub API から短命な Installation Access Token を取得する。有効期間は GitHub の仕様に従い、GATES で延長しない。 |
| FR-015 | **最小権限** を満たすよう、GitHub API の発行要求に対象リポジトリと権限を明示し、要求した対象・権限に限定する。全リポジトリ・全権限の暗黙継承を禁止する。GitHub が要求とは別に付与する権限の許容条件は FR-018 に従う。 |
| FR-016 | 成功応答には発行トークン、有効期限、対象リポジトリと実効権限、相関 ID を返す。秘密鍵・内部 GitHub App JWT は返さない。 |
| FR-017 | 新しい GitHub App Private Key へ更新でき、更新後も発行が継続する。 |
| FR-018 | GitHub API の発行応答で対象一覧・権限の欠落・不整合、要求範囲を超える対象・権限（現在許容する付随権限 `metadata:read` を除く）、不変 Owner ID の不一致を検出した場合は、トークンを返さない。応答から空でないトークン文字列を取得できた場合は失効 API の呼び出しを試み、その成否を監査する。GitHub 側の失効成功および通信断等で取得できなかったトークンの不存在は保証しない。 |

### 5.4 Usability and Operations

| ID | Requirement |
|---|---|
| FR-019 | Composite Action が AWS の認証情報の取得、認証、トークン取得の手順を利用者から隠蔽する。利用者に求める作業は、責任分界（§3）に定めたものだけとする。 |
| FR-020 | 入力不正、認証失敗、GATES による認可拒否、Installation の導出失敗、GitHub 発行 API による原因未確定の発行拒否、発行結果の不整合、外部 API・KMS の一時的な障害を区別したエラーを返す。 |
| FR-021 | 認証済み Caller（Owner ID と Repository ID、ワークフロー実行の `run_id` と `run_attempt`、ワークフロー起動属性）、要求対象、要求権限、結果、相関 ID を資格情報を含めず監査可能にする。ワークフロー起動属性は JWT に含まれる範囲で記録する。相関 ID は成功・失敗のどちらの応答にも含め、呼び出し元が監査記録と突き合わせられるようにする。 |
| FR-022 | AWS リソースの設定と変更を Terraform で管理する。秘密鍵素材の取得・変換・インポートは Terraform 管理外とする。 |
| FR-023 | 新しい鍵への切り替えでは KMS Key の意図しない置換・削除を防ぐ。 |
| FR-024 | 許可オーナー（不変 Owner ID で指定）、GitHub App、署名に用いる KMS Key は、GATES 運用者が管理するデプロイ時の設定として固定する。Installation は、固定した GitHub App と許可オーナーから一意に導出する。これらは実行時に外部入力から変更できない。 |
| FR-025 | GATES の呼び出し口は、AWS IAM で許可された主体だけが呼び出せるものとし、認証なしでインターネットに公開された呼び出し口を設けない。IAM で許可する主体は、FR-005 の不変 Owner ID で特定した許可オーナーの GitHub Actions ワークフローに限る。許可されていない主体の呼び出しは、関数が実行される前に拒否される。 |

## 6. Quality and Security Requirements

性能と可用性の数値目標は、現時点では定めません。

| ID | 観点 | Requirement |
|---|---|---|
| QR-001 | 秘密鍵保護 | GitHub Secrets、Actions Runner、Lambda のプロセスに Private Key 平文を置かない。鍵インポートは別の運用境界とする。 |
| QR-002 | 最小権限 | 利用者が明示した対象・権限に限定し、Installation の広い権限をそのまま付与しない。GitHub が要求とは別に付与する権限は FR-018 の許容条件に従う。 |
| QR-003 | Fail Closed | JWT 検証、対象解決、認可、署名、発行結果の照合が成立しなければ、利用者に Installation Access Token を返却しない。発行 API の応答を取得できない場合、GitHub 側でトークンが発行されたかは確定できず、残留トークンの不存在までは保証しない。 |
| QR-004 | 秘密情報保護 | 秘密鍵、OIDC JWT、GitHub App JWT、Installation Access Token、AWS 一時認証情報をログ・エラー本文に記録しない。 |
| QR-005 | 置換容易性 | 利用者向け Composite Action を安定した境界とする。 |
| QR-006 | 秘密鍵管理 | 秘密鍵素材を Git、Terraform State / Plan、CI の成果物へ保存しない。KMS へのインポート後は、インポート前の秘密鍵のコピーを、インポートを実施した運用者の手元から削除する。Lambda は鍵を署名にのみ使え、鍵素材のインポート・置換・削除や、鍵の管理設定の変更はできない。 |
| QR-007 | 認可の分離 | 呼び出し元の認証の成功だけでトークン発行を許可しない。対象と権限の認可を別に行う。 |

## 7. Acceptance Criteria

この表を受け入れ条件の正本とします。

| ID | Condition | Expected result | 検証する要件 |
|---|---|---|---|
| AC-01 | 許可されたオーナーのワークフローが、正当な GATES 用 OIDC JWT で `contents:read` を要求 | 指定対象だけの短命トークンを取得できる。応答に有効期限、対象、実効権限が含まれ、秘密鍵と内部 GitHub App JWT を含まない | FR-002, FR-014, FR-016 |
| AC-02 | 未認証、または許可されない呼び出し元からの要求 | 発行処理に入る前に拒否 | FR-001, FR-003 |
| AC-03 | JWT の欠落、改ざん、期限切れ、Audience 不一致、必須 Claim の欠落・不正型 | 発行を拒否 | FR-002, FR-003 |
| AC-04 | JWT の不変 Owner ID が不一致（オーナー名が一致していても） | 発行を拒否 | FR-003, FR-006 |
| AC-05 | Caller・Installation ID・任意の認可設定を Payload で指定 | 入力を受け付けず拒否 | FR-004, FR-009 |
| AC-06 | Installation の対象外、または他のオーナーの Private Repository を要求 | 発行を拒否 | FR-007, FR-010, QR-007 |
| AC-07 | `contents:write` か別の権限を要求する、または全リポジトリ（`all`）を対象にする `repository_selection` を要求に含める | 発行を拒否 | FR-008, QR-007 |
| AC-08 | リポジトリや権限が欠落、空、未知値 | 発行を拒否し、広い既定値を適用しない | FR-012 |
| AC-09 | 同一 Installation の複数対象を `contents:read` で要求 | 指定した複数対象だけへのトークンを発行 | FR-015 |
| AC-10 | 複数対象の一部が Installation の対象外、別のオーナー、または別 Installation | 全件拒否し、暗黙に分割・縮小しない | FR-007, FR-011 |
| AC-11 | App の権限上限・Installation の対象が要求より広い | 発行結果は要求した対象と権限だけに限定（FR-018 の除外対象を除く） | FR-014, FR-015, QR-002 |
| AC-12 | GitHub API の発行応答が対象・権限（FR-018 の例外を除く）の範囲外、不変 Owner ID 不一致、必要な対象一覧の欠落、または不整合 | トークンを返さない。空でないトークン文字列を取得した場合は失効 API を試み、成否を監査する。失効成功は保証しない | FR-007, FR-010, FR-018 |
| AC-13 | OIDC 公開鍵・KMS・GitHub API の取得・通信障害 | Fail Closed。資格情報を露出しない | QR-003 |
| AC-14 | GitHub App Private Key の更新と GitHub 側の旧鍵失効 | 更新後も署名と発行が成立 | FR-013, FR-017 |
| AC-15 | 利用者が `id-token: write` を宣言し、Composite Action に対象と権限、および運用者が提供する接続情報だけを渡す | 他の手順や設定なしでトークンを取得できる | FR-019 |
| AC-16 | Composite Action が発行処理のエラーを受け取る | エラー終了し、JWT・トークンをログへ出さない | FR-019, QR-004 |
| AC-17 | 入力不正、認証失敗、認可拒否、Installation の導出失敗、GitHub 発行 API の `422`、発行結果の不整合、外部 API・KMS 障害をそれぞれ発生させる | 種別ごとに区別したエラーを返す | FR-020 |
| AC-18 | 発行の成功と、認証後の拒否（認可拒否、発行結果の照合失敗など）をそれぞれ実行する | 監査記録に Caller（Owner ID、Repository ID、`run_id`、`run_attempt`、JWT に含まれていたワークフロー起動属性）、対象、権限、結果、相関 ID が残り、資格情報を含まない。成功応答にも相関 ID が含まれる | FR-016, FR-021 |
| AC-19 | 正常系と異常系を実行する | 発行処理のログとエラー本文に、秘密鍵・OIDC JWT・GitHub App JWT・トークンが含まれない | QR-004 |
| AC-20 | Terraform で AWS リソースを構築・変更 | Lambda の実行ロールが KMS の署名操作と必要なログ出力だけを許可され、鍵のインポート・置換・削除・管理設定の変更は許可されない。認証なしの呼び出し口が存在しない。意図しない KMS Key 置換・削除がない | FR-022, FR-023, FR-025, QR-006 |
| AC-21 | 秘密鍵を独立した運用で KMS にインポートし、インポート前のコピーを削除する | Terraform の State / Plan / リポジトリに鍵素材を残さず、KMS 署名が成立する。インポート前のコピーが運用者の手元に残っていない | FR-013, FR-022, FR-023, QR-006 |
| AC-22 | GitHub Secrets、Actions Runner、Lambda の設定と実行環境を確認 | GitHub App Private Key の平文が、いずれにも存在しない | QR-001 |
| AC-23 | GATES の認証・発行方式の内部実装を変更 | 利用者のワークフロー定義を変更せずに、AC-15 の条件でトークンを取得できる | QR-005 |
| AC-24 | 実行時の要求やワークフロー側の入力で、許可オーナー・GitHub App・Installation・KMS Key を指定しようとする | 指定を受け付けず、デプロイ時の設定と、そこから導出した Installation が使われる | FR-009, FR-024 |
| AC-25 | AWS の認証情報を持たない、または Invoke を許可されていない主体が Lambda を呼び出す | 関数の実行前に拒否される | FR-025 |
| AC-26 | 呼び出し元を名前だけで特定する形式のトークンを持つワークフローが、IAM ロールの引き受けを試みる | 引き受けが拒否され、Lambda は呼び出されない | FR-005 |
| AC-27 | 適切に署名された JWT で NumericDate に数字文字列を使う、`iss`、`aud`、`exp`、`iat`、`nbf` と大文字小文字だけが異なる名前の Claim を含める、`now == exp` にする、あるいは時刻に範囲の外の値か、小数部または指数部を持つ値を使う | `invalid_id_token` として拒否し、Caller を確定しない | FR-002, FR-003 |
| AC-28 | JOSE Header の未対応 `crit`、JWT の `b64:false`、2048 bit 未満の RSA 公開鍵、用途やアルゴリズムが不整合な JWK を用いた署名済み JWT | 検証鍵として採用せず `invalid_id_token` として拒否する。JWKS の取得自体の障害と、使える鍵が 1 つもない JWKS は別エラーとする | FR-002, FR-003 |
| AC-29 | 発行 API が `422` を返す | GATES 自身の認可拒否と区別した `github_issuance_rejected` として拒否し、トークンを返さない | FR-020, QR-003 |
| AC-30 | GitHub 発行 API に要求が到達した後、応答の受信に失敗してトークン文字列を取得できない | トークンを返さず、GitHub 側の発行有無・失効成功を保証しない。取得できないトークンに対する失効は要求しない | FR-018, QR-003 |
