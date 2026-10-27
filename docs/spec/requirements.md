# Requirements

GATES は、GitHub Actions ワークフローに GitHub App Installation Access Token を発行するシステムです。

## 1. Purpose

GitHub Actions が `GITHUB_TOKEN` だけでは実現できないリポジトリ間の読み取りに対し、短命な GitHub App Installation Access Token を発行します。
GitHub App Private Key を GitHub Secrets や Actions Runner に置かず、署名を AWS KMS に委譲します。

許可された単一の Organization 配下のワークフローは、GitHub App の **Installation**（GitHub App を Organization にインストールした単位）に含まれるリポジトリに対して `contents:read` のトークンをセルフサービスで要求できます。
対象リポジトリと権限を利用者に明示させ、GitHub App に付与された権限全体の払い出しを防ぎます。

利用者向けの操作は Composite Action に集約し、ワークフローが GATES の認証・発行処理の詳細に依存しない構成とします。

## 2. Scope and Constraints

### 2.1 Technical Constraints

- Go、AWS Lambda、AWS KMS を使用する。
- GitHub App Private Key は GitHub が発行するため、AWS KMS に鍵素材としてインポートして使う。
- KMS にインポートした鍵素材は、KMS から平文で取得できない。ただし、インポート前の秘密鍵コピーは KMS によって自動的に消去されない。
- AWS リソースの管理に Terraform を使う。
- 発行トークンの権限は `contents:read` に固定する。GitHub App に付与された権限が広くても、これを超えて発行しない。ただし、GitHub が要求外の権限を応答に含める場合がある。その扱いは FR-018 に従う。

### 2.2 Out of Scope

- 複数の Organization の同時サポート
- 複数の GitHub App Installation の同時サポート
- 呼び出し元を名前だけで特定する形式の GitHub OIDC トークンを持つリポジトリのサポート。該当する要求は FR-005 により拒否する。
- GitHub Enterprise Server など GitHub.com 以外の GitHub への対応
- GitHub 側の設定と操作（GitHub App の旧 Private Key の失効、Organization の OIDC トークンに不変 Owner ID を含める設定）。GATES はこれらを実施も代行もしない。
- OIDC JWT の有効期限内の再利用の検出。再利用できる期間は、FR-002 の時間条件だけで制限する。
- 連続した要求に対するレート制限と濫用の検出
- IAM 層で拒否された呼び出し（FR-025）の監査記録。記録されるかどうかは AWS の仕様に依存し、GATES は保証しない。

### 2.3 Access Boundary and Accepted Risk

**許可境界**：許可された Organization 内のいずれかのリポジトリにワークフローを追加できる人は、GATES で利用する GitHub App の Installation に含まれるリポジトリの読み取りを要求できます。
アクセス元のチームを区別したアクセス先ごとの許可は設けず、Installation の対象によって読み取り可能範囲を制限します。

**受容するリスク**：許可された Organization 内のワークフローが一つでも侵害されると、GitHub App の Installation に含まれるすべてのリポジトリの読み取りに波及します。
Installation が全リポジトリを対象にするか、選択したリポジトリだけを対象にするかは、GitHub App 側の設定に依存します。
受容の根拠は、発行する権限が `contents:read` に固定されていて、侵害されても書き込みには波及しないことです。
対象リポジトリの明示は、設定ミスによる過大な発行を防ぐ手段であり、侵害の影響範囲を狭めるものではありません。
緩和策は、Organization 管理者が Installation の対象を、GATES で読み取らせたいリポジトリだけに絞ることです。

## 3. Stakeholders and Responsibilities

| 主体 | 必要とすること | 責任 |
|---|---|---|
| ワークフロー利用者 | 必要なリポジトリと権限を指定し、運用者が用意した接続情報を参照するだけでトークンを取得できる | Composite Action を呼び出すワークフローで `permissions: id-token: write` を宣言する。Composite Action に対象リポジトリと権限、および GATES 運用者が提供する接続情報を渡す。取得したトークンを自身のワークフロー内で適切に扱う。 |
| GitHub App / Organization 管理者 | GATES を通じて読み取られ得るリポジトリを Installation の対象で管理できる | GitHub App の Installation の対象を管理する。 |
| GATES 運用者 | 秘密鍵・認可・最小権限の発行を少ない構成要素で保守できる | 認証、認可、トークン発行、秘密鍵の保護、監査を担う。GATES の呼び出しに使う IAM ロールを引き受けられる主体の条件（信頼ポリシー）を管理する。Composite Action を提供し、そのインターフェースを維持する。Composite Action に渡す接続情報（IAM ロール、Lambda 関数、AWS リージョン、GATES 用 Audience）を、ワークフローから参照できるよう GitHub 側に用意する。 |

## 4. Use Cases

| Use Case | 成立条件 |
|---|---|
| Installation に含まれる一つまたは複数のリポジトリを読み取る | 許可された呼び出し元、同一 Organization / Installation、明示された `contents:read` |
| GitHub App Private Key を更新する | 新しい鍵に切り替える。GitHub 側の旧鍵失効は対象外（§2.2） |
| 認証不正・対象範囲外・過大な要求を拒否する | JWT、リポジトリ、権限の検証が成立しなければトークンを返さない |

## 5. Functional Requirements

### 5.1 Authentication

| ID | Requirement |
|---|---|
| FR-001 | 許可された Organization の GitHub Actions ワークフローからの呼び出しだけを受け付ける。 |
| FR-002 | GATES 用 Audience の GitHub OIDC JWT の署名・Issuer・Audience・時間条件・必須 Claim を検証し、不変 Owner ID / Repository ID と実行文脈を識別する。これらで特定される呼び出し元を **Caller** と呼ぶ。Repository ID は監査のための Caller 識別に用い、認可には用いない。 |
| FR-003 | 許可されない呼び出し元、JWT の欠落・改ざん・期限切れ・Audience 不一致・不正な主体・異なる Organization を拒否する。 |
| FR-004 | 要求者の自己申告を Caller の根拠にしない。 |
| FR-005 | GATES の呼び出しに使う IAM ロールの引き受けでは、呼び出し元を不変 Owner ID で特定できる GitHub OIDC トークンだけを受け付け、名前だけで特定するトークンを拒否する。 |

### 5.2 Authorization

| ID | Requirement |
|---|---|
| FR-006 | 検証済み JWT の不変 Owner ID が、許可 Organization の不変 Owner ID と一致することを確認する。Organization 名の一致では代用しない。 |
| FR-007 | 要求先の各リポジトリが許可 Organization に属し、GATES で利用する GitHub App の同一 Installation に含まれることを確認する。 |
| FR-008 | `contents:read` のみ許可し、その他の権限や、リポジトリ単位以外（Organization 単位など）の対象の要求を拒否する。 |
| FR-009 | 利用者の入力から Caller、Installation ID、認可の根拠や任意の API Host を指定できないようにする。 |
| FR-010 | 要求されたリポジトリ名の所有者と、GitHub API が返す実効対象を検証し、要求外のリポジトリへ発行しない。 |
| FR-011 | 複数対象の要求は全件が成立する場合のみ許可し、一部成功や要求範囲の自動的な縮小を行わない。 |
| FR-012 | 利用者に対象リポジトリと権限の明示を要求し、欠落・未知値を広い既定権限に読み替えない。 |

### 5.3 Token Issuance

| ID | Requirement |
|---|---|
| FR-013 | 秘密鍵の平文を取得せずに GitHub App JWT を生成する。 |
| FR-014 | GitHub App Installation に付与された対象・権限を上限として、GitHub API から短命な Installation Access Token を取得する。有効期間は GitHub の仕様に従い、GATES で延長しない。 |
| FR-015 | **最小権限** を満たすよう、GitHub API の発行要求に対象リポジトリと権限を明示し、要求した対象・権限だけを発行する。全リポジトリ・全権限の暗黙継承を禁止する。 |
| FR-016 | 成功応答には発行トークン、有効期限、対象リポジトリと実効権限を返す。秘密鍵・内部 GitHub App JWT は返さない。 |
| FR-017 | 新しい GitHub App Private Key へ更新でき、更新後も発行が継続する。 |
| FR-018 | GitHub API 応答の対象一覧・権限が欠落または不整合、要求の対象または権限（要求にかかわらず GitHub が付与する権限を除く）を超過、不変 Owner ID が不一致の場合はトークンを返さない。 |

### 5.4 Usability and Operations

| ID | Requirement |
|---|---|
| FR-019 | Composite Action が AWS の認証情報の取得、認証、トークン取得の手順を利用者から隠蔽する。利用者に求める作業は、責任分界（§3）に定めたものだけとする。 |
| FR-020 | 入力不正、認証失敗、認可拒否、外部 API・KMS 障害を区別したエラーを返す。 |
| FR-021 | 認証済み Caller（Owner ID と Repository ID）、要求対象、要求権限、結果、相関 ID を資格情報を含めず監査可能にする。 |
| FR-022 | AWS リソースの設定と変更を Terraform で管理する。秘密鍵素材の取得・変換・インポートは Terraform 管理外とする。 |
| FR-023 | 新しい鍵への切り替えでは KMS Key の意図しない置換・削除を防ぐ。 |
| FR-024 | 許可 Organization（不変 Owner ID で指定）、GitHub App、署名に用いる KMS Key は、GATES 運用者が管理するデプロイ時の設定として固定する。Installation は、固定した GitHub App と許可 Organization から一意に導出する。これらは実行時に外部入力から変更できない。 |
| FR-025 | GATES の呼び出し口は、AWS IAM で許可された主体だけが呼び出せるものとし、認証なしでインターネットに公開された呼び出し口を設けない。IAM で許可する主体は、FR-005 の不変 Owner ID で特定した許可 Organization の GitHub Actions ワークフローに限る。許可されていない主体の呼び出しは、関数が実行される前に拒否される。 |

## 6. Quality and Security Requirements

性能と可用性の数値目標は、現時点では定めません。

| ID | 観点 | Requirement |
|---|---|---|
| QR-001 | 秘密鍵保護 | GitHub Secrets、Actions Runner、Lambda のプロセスに Private Key 平文を置かない。鍵インポートは別の運用境界とする。 |
| QR-002 | 最小権限 | 利用者が明示した対象・権限を上限とし、Installation の広い権限をそのまま付与しない。 |
| QR-003 | Fail Closed | JWT 検証、対象解決、認可、署名、発行結果の検証ができなければ発行しない。 |
| QR-004 | 秘密情報保護 | 秘密鍵、OIDC JWT、GitHub App JWT、Installation Token をログ・エラー本文に記録しない。 |
| QR-005 | 置換容易性 | 利用者向け Composite Action を安定した境界とする。 |
| QR-006 | 秘密鍵管理 | 秘密鍵素材を Git、Terraform State / Plan、CI の成果物へ保存しない。KMS へのインポート後は、インポート前の秘密鍵のコピーを、インポートを実施した運用者の手元から削除する。Lambda は鍵を署名にのみ使え、鍵素材のインポート・置換・削除や、鍵の管理設定の変更はできない。 |
| QR-007 | 認可の分離 | 呼び出し元の認証の成功だけでトークン発行を許可しない。対象と権限の認可を別に行う。 |

## 7. Acceptance Criteria

この表を受け入れ条件の正本とします。

| ID | Condition | Expected result | 検証する要件 |
|---|---|---|---|
| AC-01 | 許可された Organization のワークフローが、正当な GATES 用 OIDC JWT で `contents:read` を要求 | 指定対象だけの短命トークンを取得できる。応答に有効期限、対象、実効権限が含まれ、秘密鍵と内部 GitHub App JWT を含まない | FR-002, FR-014, FR-016 |
| AC-02 | 未認証、または許可されない呼び出し元からの要求 | 発行処理に入る前に拒否 | FR-001, FR-003 |
| AC-03 | JWT の欠落、改ざん、期限切れ、Audience 不一致、必須 Claim の欠落 | 発行を拒否 | FR-002, FR-003 |
| AC-04 | JWT の不変 Owner ID が不一致（Organization 名が一致していても） | 発行を拒否 | FR-003, FR-006 |
| AC-05 | Caller・Installation ID・任意の認可設定を Payload で指定 | 入力を受け付けず拒否 | FR-004, FR-009 |
| AC-06 | Installation の対象外、または他 Organization の Private Repository を要求 | 発行を拒否 | FR-007, FR-010, QR-007 |
| AC-07 | `contents:write`、別の権限、Organization 単位の対象を要求 | 発行を拒否 | FR-008, QR-007 |
| AC-08 | リポジトリや権限が欠落、空、未知値 | 発行を拒否し、広い既定値を適用しない | FR-012 |
| AC-09 | 同一 Installation の複数対象を `contents:read` で要求 | 指定した複数対象だけへのトークンを発行 | FR-015 |
| AC-10 | 複数対象の一部が Installation の対象外、別 Organization、または別 Installation | 全件拒否し、暗黙に分割・縮小しない | FR-007, FR-011 |
| AC-11 | App の権限上限・Installation の対象が要求より広い | 発行結果は要求した対象と権限だけに限定（FR-018 の除外対象を除く） | FR-014, FR-015, QR-002 |
| AC-12 | GitHub API の発行応答が対象・権限（FR-018 の除外対象を除く）の範囲外、不変 Owner ID 不一致、必要な対象一覧の欠落、または不整合 | トークンを返さない | FR-010, FR-018 |
| AC-13 | OIDC 公開鍵・KMS・GitHub API の取得・通信障害 | Fail Closed。資格情報を露出しない | QR-003 |
| AC-14 | GitHub App Private Key の更新と GitHub 側の旧鍵失効 | 更新後も署名と発行が成立 | FR-013, FR-017 |
| AC-15 | 利用者が `id-token: write` を宣言し、Composite Action に対象と権限、および運用者が提供する接続情報だけを渡す | 他の手順や設定なしでトークンを取得できる | FR-019 |
| AC-16 | Composite Action が発行処理のエラーを受け取る | エラー終了し、JWT・トークンをログへ出さない | FR-019, QR-004 |
| AC-17 | 入力不正、認証失敗、認可拒否、外部 API・KMS 障害をそれぞれ発生させる | 種別ごとに区別したエラーを返す | FR-020 |
| AC-18 | 発行の成功と、認証後の拒否（認可拒否、発行結果の検証失敗など）をそれぞれ実行する | 監査記録に Caller（Owner ID と Repository ID）、対象、権限、結果、相関 ID が残り、資格情報を含まない | FR-021 |
| AC-19 | 正常系と異常系を実行する | 発行処理のログとエラー本文に、秘密鍵・OIDC JWT・GitHub App JWT・トークンが含まれない | QR-004 |
| AC-20 | Terraform で AWS リソースを構築・変更 | Lambda の実行ロールが KMS の署名操作と必要なログ出力だけを許可され、鍵のインポート・置換・削除・管理設定の変更は許可されない。認証なしの呼び出し口が存在しない。意図しない KMS Key 置換・削除がない | FR-022, FR-023, FR-025, QR-006 |
| AC-21 | 秘密鍵を独立した運用で KMS にインポートし、インポート前のコピーを削除する | Terraform の State / Plan / リポジトリに鍵素材を残さず、KMS 署名が成立する。インポート前のコピーが運用者の手元に残っていない | FR-013, FR-022, FR-023, QR-006 |
| AC-22 | GitHub Secrets、Actions Runner、Lambda の設定と実行環境を確認 | GitHub App Private Key の平文が、いずれにも存在しない | QR-001 |
| AC-23 | GATES の認証・発行方式の内部実装を変更 | 利用者のワークフロー定義を変更せずに、AC-15 の条件でトークンを取得できる | QR-005 |
| AC-24 | 実行時の要求やワークフロー側の入力で、許可 Organization・GitHub App・Installation・KMS Key を指定しようとする | 指定を受け付けず、デプロイ時の設定と、そこから導出した Installation が使われる | FR-009, FR-024 |
| AC-25 | AWS の認証情報を持たない、または Invoke を許可されていない主体が Lambda を呼び出す | 関数の実行前に拒否される | FR-025 |
| AC-26 | 呼び出し元を名前だけで特定する形式のトークンを持つワークフローが、IAM ロールの引き受けを試みる | 引き受けが拒否され、Lambda は呼び出されない | FR-005 |
