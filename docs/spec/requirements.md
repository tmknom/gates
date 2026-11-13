# Requirements

GATES は、GitHub Actions ワークフローに GitHub App Installation Access Token を発行するシステムです。

## 1. Purpose

本ドキュメントは、現在実現すべき要求と受け入れ条件を定めます。

GitHub Actions が `GITHUB_TOKEN` だけでは実現できないリポジトリ間の読み取りに対し、短命な GitHub App Installation Access Token を発行します。
GitHub App Private Key を GitHub Secrets や Actions Runner に置かず、署名を AWS KMS に委譲します。

許可された単一の **オーナー**（リポジトリを所有する GitHub の Organization またはユーザーアカウント）配下のワークフローは、GitHub App の **Installation**（GitHub App をオーナーにインストールした単位）に含まれるリポジトリに対して `contents:read` のトークンをセルフサービスで要求できます。
対象リポジトリと権限を利用者に明示させ、GitHub App に付与された権限全体の払い出しを防ぎます。

利用者向けの操作は Docker Action に集約し、ワークフローが GATES の認証・発行処理の詳細に依存しない構成とします。

## 2. Scope and Constraints

### 2.1 Technical Constraints

- Go、AWS Lambda、AWS KMS を使用する。
- GitHub App Private Key は GitHub が発行するため、AWS KMS に鍵素材としてインポートして使う。
- KMS にインポートした鍵素材は、KMS から平文で取得できない。ただし、インポート前の秘密鍵コピーは KMS によって自動的に消去されない。
- AWS リソースの管理に Terraform を使う。
- 現在のスコープで発行を許可する権限は `contents:read` に固定する。GitHub App / Installation に付与された権限が広くても、GATES の許可条件を超えて発行しない。GitHub が要求とは別に付与する権限の扱いは FR-018 に従う。

### 2.2 Out of Scope

- 複数のオーナーの同時サポート
- 複数の GitHub App Installation の同時サポート
- 呼び出し元を名前だけで特定する形式の GitHub OIDC JWT を持つリポジトリのサポート。該当する要求は FR-005 により拒否する。
- OIDC JWT の `sub` のカスタマイズにより、`repo:OWNER@OWNER-ID/REPO@REPO-ID:` で始まる形式から外れるリポジトリのサポート。該当する要求は拒否する。
- GitHub Enterprise Server など GitHub.com 以外の GitHub への対応
- GitHub 側の設定と操作（GitHub App の旧 Private Key の失効、オーナー配下のリポジトリの OIDC JWT に不変 Owner ID を含める設定）。GATES はこれらを実施も代行もしない。
- OIDC JWT の有効期限内の再利用の検出。再利用できる期間は、FR-002 の時間条件だけで制限する。
- 連続した要求に対するレート制限と濫用の検出
- IAM 層で拒否された呼び出し（FR-025）の監査記録。記録されるかどうかは AWS の仕様に依存し、GATES は保証しない。

### 2.3 Current Access Boundary

現在のスコープでは、許可オーナーのワークフローは、同一の GitHub App Installation に含まれるリポジトリへの `contents:read` を要求できます。Caller 別・チーム別にアクセス先を限定する認可は設けません。対象と権限は要求に明示させ、GATES の許可条件と Installation の上限で制限します。

## 3. Stakeholders and Responsibilities

| 主体 | 必要とすること | 責任 |
|---|---|---|
| ワークフロー利用者 | 必要なリポジトリと権限を指定し、運用者が用意した接続情報を参照するだけでトークンを取得できる | Docker Action を呼び出すワークフローで `permissions: id-token: write` を宣言する。Docker Action に対象リポジトリと権限、および GATES 運用者が提供する接続情報を渡す。セルフホストの Runner で実行する場合は、Linux と Docker を Runner に用意し、イメージのビルドと Docker Action の処理に要る宛先（Docker Hub、Go module proxy、GitHub、AWS）への通信を許可する。取得したトークンを自身のワークフロー内で適切に扱う。 |
| GitHub App / オーナーの管理者 | GATES を通じて読み取られ得るリポジトリを Installation の対象で管理できる | GitHub App の Installation の対象を管理する。 |
| GATES 運用者 | 秘密鍵・認可・最小権限の発行を少ない構成要素で保守できる | 認証、認可、トークン発行、秘密鍵の保護、監査を担う。GATES の呼び出しに使う IAM Role を引き受けられる主体の条件（信頼ポリシー）を管理する。Docker Action を提供し、そのインターフェースを維持する。Docker Action に渡す接続情報（IAM Role、Lambda 関数、AWS リージョン、GATES 用 Audience）を、ワークフローから参照できるよう GitHub 側に用意する。 |

## 4. Use Cases

| Use Case | 成立条件 |
|---|---|
| Installation に含まれる一つまたは複数のリポジトリを読み取る | 許可された呼び出し元、同一オーナー / Installation、明示された `contents:read` |
| GitHub App Private Key を更新する | 新しい鍵に切り替える。GitHub 側の旧鍵失効は対象外（§2.2） |
| 認証不正・対象範囲外・過大な要求を拒否する | JWT、リポジトリ、権限の検証が成立しなければトークンを返さない |

## 5. Functional Requirements

### 5.1 Authentication

#### FR-001 許可オーナーのワークフローに限る呼び出しの受け付け

許可されたオーナーの GitHub Actions ワークフローからの呼び出しだけを受け付ける。

#### FR-002 GitHub OIDC JWT の検証と Caller の識別

GATES 用 Audience の GitHub OIDC JWT の署名・Issuer・Audience・時間条件・必須 Claim を検証し、不変 Owner ID / Repository ID で呼び出し元を識別する。これで特定される呼び出し元を **Caller** と呼ぶ。Repository ID は監査のための Caller 識別に用い、認可には用いない。ワークフロー実行 ID と試行番号、およびワークフロー起動属性（起動したイベント、ref、ワークフロー、environment、行為者など）は、監査のためだけに Caller に添え、認可には用いない。ワークフロー起動属性は JWT に含まれる範囲で読み取り、欠落を拒否の理由にしない。

#### FR-003 不正な JWT と許可されない呼び出し元の拒否

許可されない呼び出し元、JWT の欠落・改ざん・期限切れ・Audience 不一致・不正な主体・異なるオーナーを拒否する。

#### FR-004 Caller の根拠としての自己申告の不採用

要求者の自己申告を Caller の根拠にしない。

#### FR-005 不変 Owner ID で呼び出し元を特定できる JWT に限る IAM Role の引き受け

GATES の呼び出しに使う IAM Role の引き受けでは、呼び出し元を不変 Owner ID で特定できる GitHub OIDC JWT だけを受け付け、名前だけで特定する JWT を拒否する。

### 5.2 Authorization

#### FR-006 許可オーナーの Owner ID による認可

検証済み JWT の不変 Owner ID が、許可オーナーの不変 Owner ID と一致することを確認する。オーナー名の一致では代用しない。

#### FR-007 同一 Installation に含まれるリポジトリへの発行の限定

許可オーナーに属し、GATES で利用する GitHub App の同一 Installation に含まれるリポジトリだけに発行する。含まれない対象を一つでも含む要求にはトークンを返さない。発行前に確認するか発行結果で検出するかは設計に委ねる。発行結果で検出し、トークン文字列を取得できた場合は失効を試みる。

#### FR-008 権限の `contents:read` への限定と、要求で明示したリポジトリへの対象の限定

現在のスコープでは `contents:read` のみ許可し、その他の権限を拒否する。対象は要求で明示したリポジトリ（GitHub の `repository_selection` でいう `selected`）に限り、全リポジトリ（`all`）を対象にする手段を利用者に与えない。

#### FR-009 利用者の入力による Caller、Installation ID、認可の根拠、API Host の指定の禁止

利用者の入力から Caller、Installation ID、認可の根拠や任意の API Host を指定できないようにする。

#### FR-010 要求したリポジトリのオーナーと、GitHub API が返す実効対象の検証

要求されたリポジトリ名のオーナーと、GitHub API が返す実効対象を検証し、要求外のリポジトリへ発行しない。

#### FR-011 複数対象の要求の全件成立

複数対象の要求は全件が成立する場合のみ許可し、一部成功や要求範囲の自動的な縮小を行わない。

#### FR-012 利用者による対象リポジトリと権限の明示

利用者に対象リポジトリと権限の明示を要求し、欠落・未知値を広い既定権限に読み替えない。

### 5.3 Token Issuance

#### FR-013 秘密鍵の平文を取得しない GitHub App JWT の生成

秘密鍵の平文を取得せずに GitHub App JWT を生成する。

#### FR-014 Installation の範囲を上限とする短命なトークンの取得

GitHub App Installation に付与された対象・権限を上限として、GitHub API から短命な Installation Access Token を取得する。有効期間は GitHub の仕様に従い、GATES で延長しない。

#### FR-015 発行 API の呼び出しでの対象と権限の明示

**最小権限** を満たすよう、発行 API の呼び出しに対象リポジトリと権限を明示し、要求した対象・権限に限定する。全リポジトリ・全権限の暗黙継承を禁止する。GitHub が要求とは別に付与する権限の許容条件は FR-018 に従う。

#### FR-016 成功応答の内容

成功応答には Installation Access Token、有効期限、対象リポジトリと実効権限、相関 ID を返す。秘密鍵・内部 GitHub App JWT は返さない。

#### FR-017 GitHub App Private Key の更新

新しい GitHub App Private Key へ更新でき、更新後も発行が継続する。

#### FR-018 発行応答の照合と、照合に失敗したときのトークンの失効

GitHub API の発行応答で対象一覧・権限の欠落・不整合、要求範囲を超える対象・権限（現在許容する付随権限 `metadata:read` を除く）、不変 Owner ID の不一致を検出した場合は、トークンを返さない。応答から空でないトークン文字列を取得できた場合は失効 API の呼び出しを試み、その成否を監査する。GitHub 側の失効成功および通信断等で取得できなかったトークンの不存在は保証しない。

#### FR-026 Docker Action によるトークンの失効

Docker Action は、受け取った Installation Access Token の失効を試み、その成否をワークフローのログに出す。後続ステップへ渡したトークンはジョブの終了時に、受け取った後、後続ステップへ渡す前に失敗した場合はその時点で失効を試みる。ジョブがキャンセルされた場合も、受け取ったトークンの失効をこのどちらかで試みるが、その成否がワークフローのログに残ることは保証しない。失効はベストエフォートとし、失効の失敗でジョブを失敗させない。トークンを受け取る前に失敗した場合は失効を試みない。利用者が失効を止める手段は設けない。

### 5.4 Usability and Operations

#### FR-019 Docker Action による取得手順の隠蔽

Docker Action が AWS の認証情報の取得、認証、トークン取得の手順を利用者から隠蔽する。利用者に求める作業は、責任分界（§3）に定めたものだけとする。

#### FR-020 失敗の種別を区別したエラー

入力不正、認証失敗、GATES による認可拒否、Installation 利用不可（発行に使える Installation を得られない）、原因が未確定の GitHub 発行拒否、発行結果の照合失敗、外部 API・KMS の一時的な障害を区別したエラーを返す。

#### FR-021 資格情報を含まない監査記録

認証済み Caller（Owner ID と Repository ID、ワークフロー実行 ID と試行番号、ワークフロー起動属性）、要求対象、要求権限、結果、相関 ID を資格情報を含めず監査可能にする。ワークフロー起動属性は JWT に含まれる範囲で記録する。相関 ID は成功・失敗のどちらの応答にも含め、呼び出し元が監査記録と突き合わせられるようにする。

#### FR-022 Terraform による AWS リソースの管理

AWS リソースの設定と変更を Terraform で管理する。秘密鍵素材の取得・変換・インポートと、IAM OIDC Provider の用意は Terraform 管理外とする。

#### FR-023 鍵の切り替えでの KMS Key の意図しない置換と削除の防止

新しい鍵への切り替えでは KMS Key の意図しない置換・削除を防ぐ。

#### FR-024 デプロイ時の設定による許可オーナー、GitHub App、KMS Key の固定

許可オーナー（不変 Owner ID で指定）、GitHub App、署名に用いる KMS Key は、GATES 運用者が管理するデプロイ時の設定として固定する。Installation は、固定した GitHub App と許可オーナーから一意に導出する。これらは実行時に外部入力から変更できない。

#### FR-025 IAM で許可された主体に限る呼び出し口

GATES の呼び出し口は、AWS IAM で許可された主体だけが呼び出せるものとし、認証なしでインターネットに公開された呼び出し口を設けない。IAM で許可する主体は、FR-005 の不変 Owner ID で特定した許可オーナーの GitHub Actions ワークフローに限る。許可されていない主体の呼び出しは、関数が実行される前に拒否される。

## 6. Quality and Security Requirements

性能と可用性の数値目標は、現時点では定めません。

### QR-001 秘密鍵保護

GitHub Secrets、Actions Runner、Lambda のプロセスに Private Key 平文を置かない。鍵インポートは別の運用境界とする。

### QR-002 最小権限

利用者が明示した対象・権限に限定し、Installation の広い権限をそのまま付与しない。GitHub が要求とは別に付与する権限は FR-018 の許容条件に従う。

### QR-003 Fail Closed

JWT 検証、Installation の解決、認可、署名、発行結果の照合が成立しなければ、利用者に Installation Access Token を返却しない。発行 API の応答を取得できない場合、GitHub 側でトークンが発行されたかは確定できず、残留トークンの不存在までは保証しない。

### QR-004 秘密情報保護

秘密鍵、OIDC JWT、GitHub App JWT、Installation Access Token、AWS 一時認証情報をログ・エラー本文に記録しない。

### QR-005 置換容易性

利用者向け Docker Action を安定した境界とする。

### QR-006 秘密鍵管理

秘密鍵素材を Git、Terraform State / Plan、CI の成果物へ保存しない。KMS へのインポート後は、インポート前の秘密鍵のコピーを、インポートを実施した運用者の手元から削除する。Lambda は鍵を署名にのみ使え、鍵素材のインポート・置換・削除や、鍵の管理設定の変更はできない。

### QR-007 認可の分離

呼び出し元の認証の成功だけでトークン発行を許可しない。対象と権限の認可を別に行う。

## 7. Acceptance Criteria

本節の各 AC を受け入れ条件の正本とします。
各 AC の Relationship の表は、その AC が適合を判定する要求を `alignment` で示します。

### AC-01 許可オーナーの正当な要求に対するトークンの発行

**条件**：許可されたオーナーのワークフローが、正当な GATES 用 OIDC JWT で `contents:read` を要求

**期待する結果**：指定対象だけの短命トークンを取得できる。応答に有効期限、対象、実効権限が含まれ、秘密鍵と内部 GitHub App JWT を含まない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-002` | Functional Requirement |
| `alignment` | `FR-014` | Functional Requirement |
| `alignment` | `FR-016` | Functional Requirement |

### AC-02 未認証か許可されない呼び出し元の要求の拒否

**条件**：未認証、または許可されない呼び出し元からの要求

**期待する結果**：発行処理に入る前に拒否

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-003` | Functional Requirement |

### AC-03 JWT の欠落と、改ざん、期限切れ、Audience 不一致、必須 Claim の欠落・不正型の JWT の拒否

**条件**：JWT の欠落、改ざん、期限切れ、Audience 不一致、必須 Claim の欠落・不正型

**期待する結果**：発行を拒否

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-002` | Functional Requirement |
| `alignment` | `FR-003` | Functional Requirement |

### AC-04 オーナー名が一致して Owner ID が異なる JWT の拒否

**条件**：JWT の不変 Owner ID が不一致（オーナー名が一致していても）

**期待する結果**：発行を拒否

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-003` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |

### AC-05 Payload での Caller、Installation ID、認可設定の指定の拒否

**条件**：Caller・Installation ID・任意の認可設定を Payload で指定

**期待する結果**：入力を受け付けず拒否

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `FR-009` | Functional Requirement |

### AC-06 Installation の対象外のリポジトリか、他のオーナーの Private Repository の要求の拒否

**条件**：Installation の対象外、または他のオーナーの Private Repository を要求

**期待する結果**：発行を拒否

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-007` | Functional Requirement |
| `alignment` | `FR-010` | Functional Requirement |
| `alignment` | `QR-007` | Quality Requirement |

### AC-07 `contents:read` 以外の権限と全リポジトリの要求の拒否

**条件**：`contents:write` か別の権限を要求する、または全リポジトリ（`all`）を対象にする `repository_selection` を要求に含める

**期待する結果**：発行を拒否

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-008` | Functional Requirement |
| `alignment` | `QR-007` | Quality Requirement |

### AC-08 リポジトリか権限が欠落、空、未知値の要求の拒否

**条件**：リポジトリや権限が欠落、空、未知値

**期待する結果**：発行を拒否し、広い既定値を適用しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-012` | Functional Requirement |

### AC-09 同一 Installation の複数対象への発行

**条件**：同一 Installation の複数対象を `contents:read` で要求

**期待する結果**：指定した複数対象だけへのトークンを発行

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-015` | Functional Requirement |

### AC-10 一部が発行できない複数対象の要求の全件拒否

**条件**：複数対象の一部が Installation の対象外、別のオーナー、または別 Installation

**期待する結果**：全件拒否し、暗黙に分割・縮小しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-007` | Functional Requirement |
| `alignment` | `FR-011` | Functional Requirement |

### AC-11 App と Installation の範囲が要求より広い場合の発行範囲の限定

**条件**：App の権限上限・Installation の対象が要求より広い

**期待する結果**：発行結果は要求した対象と権限だけに限定（FR-018 の除外対象を除く）

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-014` | Functional Requirement |
| `alignment` | `FR-015` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |

### AC-12 照合に失敗した発行応答でのトークンの不返却と失効の試行

**条件**：GitHub API の発行応答が対象・権限（FR-018 の例外を除く）の範囲外、不変 Owner ID 不一致、必要な対象一覧の欠落、または不整合

**期待する結果**：トークンを返さない。空でないトークン文字列を取得した場合は失効 API を試み、成否を監査する。失効成功は保証しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-007` | Functional Requirement |
| `alignment` | `FR-010` | Functional Requirement |
| `alignment` | `FR-018` | Functional Requirement |

### AC-13 OIDC 公開鍵、KMS、GitHub API の障害での Fail Closed

**条件**：OIDC 公開鍵・KMS・GitHub API の取得・通信障害

**期待する結果**：Fail Closed。資格情報を露出しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `QR-003` | Quality Requirement |

### AC-14 GitHub App Private Key の更新後の発行

**条件**：GitHub App Private Key の更新と GitHub 側の旧鍵失効

**期待する結果**：更新後も署名と発行が成立

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-013` | Functional Requirement |
| `alignment` | `FR-017` | Functional Requirement |

### AC-15 対象、権限、接続情報だけを Docker Action に渡したときのトークンの取得

**条件**：GitHub がホストする Runner で、利用者が `id-token: write` を宣言し、Docker Action に対象と権限、および運用者が提供する接続情報だけを渡す

**期待する結果**：他の手順や設定なしでトークンを取得できる

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-019` | Functional Requirement |

### AC-16 Docker Action が発行処理のエラーを受け取ったときの終了

**条件**：Docker Action が発行処理のエラーを受け取る

**期待する結果**：エラー終了し、JWT・トークンをログへ出さない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-019` | Functional Requirement |
| `alignment` | `QR-004` | Quality Requirement |

### AC-17 失敗の種別ごとに区別したエラー

**条件**：入力不正、認証失敗、認可拒否、Installation 利用不可、GitHub 発行 API の `422`、発行結果の照合失敗、外部 API・KMS の一時的な障害をそれぞれ発生させる

**期待する結果**：種別ごとに区別したエラーを返す

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-020` | Functional Requirement |

### AC-18 成功と認証後の拒否の監査記録

**条件**：発行の成功と、認証後の拒否（認可拒否、発行結果の照合失敗など）をそれぞれ実行する

**期待する結果**：監査記録に Caller（Owner ID、Repository ID、ワークフロー実行 ID、試行番号、JWT に含まれていたワークフロー起動属性）、対象、権限、結果、相関 ID が残り、資格情報を含まない。成功応答にも相関 ID が含まれる

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-016` | Functional Requirement |
| `alignment` | `FR-021` | Functional Requirement |

### AC-19 ログとエラー本文からの秘密情報の除外

**条件**：正常系と異常系を実行する

**期待する結果**：発行処理のログとエラー本文に、秘密鍵・OIDC JWT・GitHub App JWT・トークンが含まれない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `QR-004` | Quality Requirement |

### AC-20 Terraform で構築した AWS リソースでの Lambda の実行 Role の権限の限定、認証なしの呼び出し口の不在、KMS Key の意図しない置換と削除の防止

**条件**：Terraform で AWS リソースを構築・変更

**期待する結果**：Lambda の実行 Role が KMS の署名操作と必要なログ出力だけを許可され、鍵のインポート・置換・削除・管理設定の変更は許可されない。認証なしの呼び出し口が存在しない。意図しない KMS Key 置換・削除がない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-022` | Functional Requirement |
| `alignment` | `FR-023` | Functional Requirement |
| `alignment` | `FR-025` | Functional Requirement |
| `alignment` | `QR-006` | Quality Requirement |

### AC-21 秘密鍵のインポートとインポート前のコピーの削除

**条件**：秘密鍵を独立した運用で KMS にインポートし、インポート前のコピーを削除する

**期待する結果**：Terraform の State / Plan / リポジトリに鍵素材を残さず、KMS 署名が成立する。インポート前のコピーが運用者の手元に残っていない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-013` | Functional Requirement |
| `alignment` | `FR-022` | Functional Requirement |
| `alignment` | `FR-023` | Functional Requirement |
| `alignment` | `QR-006` | Quality Requirement |

### AC-22 GitHub App Private Key の平文の不在

**条件**：GitHub Secrets、Actions Runner、Lambda の設定と実行環境を確認

**期待する結果**：GitHub App Private Key の平文が、いずれにも存在しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `QR-001` | Quality Requirement |

### AC-23 内部実装の変更に対する利用者のワークフローの維持

**条件**：GATES の認証・発行方式の内部実装を変更

**期待する結果**：利用者のワークフロー定義を変更せずに、AC-15 の条件でトークンを取得できる

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `QR-005` | Quality Requirement |

### AC-24 実行時の入力による許可オーナー、GitHub App、Installation、KMS Key の指定の拒否

**条件**：実行時の要求やワークフロー側の入力で、許可オーナー・GitHub App・Installation・KMS Key を指定しようとする

**期待する結果**：指定を受け付けず、デプロイ時の設定と、そこから導出した Installation が使われる

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-009` | Functional Requirement |
| `alignment` | `FR-024` | Functional Requirement |

### AC-25 Invoke を許可されていない主体による Lambda の呼び出しの拒否

**条件**：AWS の認証情報を持たない、または Invoke を許可されていない主体が Lambda を呼び出す

**期待する結果**：関数の実行前に拒否される

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-025` | Functional Requirement |

### AC-26 名前だけで呼び出し元を特定する JWT による IAM Role の引き受けの拒否

**条件**：呼び出し元を名前だけで特定する形式の OIDC JWT を持つワークフローが、IAM Role の引き受けを試みる

**期待する結果**：引き受けが拒否され、Lambda は呼び出されない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-005` | Functional Requirement |

### AC-27 Case Folding で一致する別名の Claim と不適格な時刻の値を持つ JWT の拒否

**条件**：適切に署名された JWT で NumericDate に数字文字列を使う、`iss`、`aud`、`exp`、`iat`、`nbf` と Unicode の単純な Case Folding で一致する別の名前（`ISS`、`iſſ` など）の Claim を含める、`now == exp` にする、あるいは時刻に範囲の外の値か、小数部または指数部を持つ値を使う

**期待する結果**：認証失敗として拒否し、Caller を確定しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-002` | Functional Requirement |
| `alignment` | `FR-003` | Functional Requirement |

### AC-28 未対応の JOSE Header の拡張と不適格な検証鍵を用いた JWT の拒否

**条件**：JOSE Header の未対応 `crit`、JWT の `b64:false`、2048 bit 未満の RSA 公開鍵、用途やアルゴリズムが不整合な JWK を用いた署名済み JWT

**期待する結果**：検証鍵として採用せず認証失敗として拒否する。JWKS の取得自体の障害と、使用可能な鍵が 1 つもない JWKS は、認証失敗とは別のエラーとする

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-002` | Functional Requirement |
| `alignment` | `FR-003` | Functional Requirement |

### AC-29 発行 API の `422` の GitHub 発行拒否としての扱い

**条件**：発行 API が `422` を返す

**期待する結果**：GATES 自身の認可拒否と区別した GitHub 発行拒否として扱い、トークンを返さない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-020` | Functional Requirement |
| `alignment` | `QR-003` | Quality Requirement |

### AC-30 発行 API の応答を受信できない場合の扱い

**条件**：GitHub 発行 API に要求が到達した後、応答の受信に失敗してトークン文字列を取得できない

**期待する結果**：トークンを返さず、GitHub 側の発行有無・失効成功を保証しない。取得できないトークンに対する失効は要求しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-018` | Functional Requirement |
| `alignment` | `QR-003` | Quality Requirement |

### AC-31 ジョブの終了時のトークンの失効

**条件**：Docker Action がトークンを後続ステップへ渡したジョブが終了する

**期待する結果**：ジョブの終了時に Docker Action がそのトークンの失効を試み、失効の成否をワークフローのログに出し、トークンをログに出さない。失効が失敗してもジョブの結果は変わらない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-026` | Functional Requirement |
| `alignment` | `QR-004` | Quality Requirement |

### AC-32 後続ステップへ渡す前に失敗した場合のトークンの失効

**条件**：Docker Action がトークンを受け取った後、成功応答の判定か、Output か `GITHUB_STATE` への書き込みに失敗する

**期待する結果**：Docker Action がその時点でトークンの失効を試み、失効の成否をワークフローのログに出し、トークンをログに出さずにエラー終了する

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-026` | Functional Requirement |
| `alignment` | `QR-004` | Quality Requirement |

### AC-33 ジョブのキャンセル時のトークンの失効

**条件**：Docker Action が Lambda の Invoke を始めた後、メインの処理を終える前にジョブがキャンセルされる

**期待する結果**：Docker Action はトークンを受け取った場合、後続ステップへ渡す前ならその時点で、渡した後ならジョブの終了時に、トークンの失効を試み、トークンをログに出さない。Runner が Docker Action を強制終了した場合は失効の試行を保証しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-026` | Functional Requirement |
| `alignment` | `QR-004` | Quality Requirement |
