# Requirements

GATES は、GitHub Actions ワークフローに GitHub App Installation Access Token を発行するシステムです。

## 1. Purpose

本ドキュメントは、現在実現すべき要求を定めます。

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
- 現在のスコープで発行を許可する権限は `contents:read` に固定する。GitHub App / Installation に付与された権限が広くても、GATES の許可条件を超えて発行しない。GitHub が要求とは別に付与する権限の扱いは FR-ISSUANCE-006 に従う。

### 2.2 Out of Scope

- 複数のオーナーの同時サポート
- 複数の GitHub App Installation の同時サポート
- 呼び出し元を名前だけで特定する形式の GitHub OIDC JWT を持つリポジトリのサポート。該当する要求は FR-AUTHN-005 により拒否する。
- OIDC JWT の `sub` のカスタマイズにより、`repo:OWNER@OWNER-ID/REPO@REPO-ID:` で始まる形式から外れるリポジトリのサポート。該当する要求は拒否する。
- GitHub Enterprise Server など GitHub.com 以外の GitHub への対応
- GitHub 側の設定と操作（GitHub App の旧 Private Key の失効、オーナー配下のリポジトリの OIDC JWT に不変 Owner ID を含める設定）。GATES はこれらを実施も代行もしない。
- OIDC JWT の有効期限内の再利用の検出。再利用できる期間は、FR-AUTHN-002 の時間条件だけで制限する。
- 連続した要求に対するレート制限と濫用の検出
- IAM 層で拒否された呼び出し（FR-OPS-007）の監査記録。記録されるかどうかは AWS の仕様に依存し、GATES は保証しない。

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

#### FR-AUTHN-001 許可オーナーのワークフローに限る呼び出しの受け付け

許可されたオーナーの GitHub Actions ワークフローからの呼び出しだけを受け付ける。

#### FR-AUTHN-002 GitHub OIDC JWT の検証と Caller の識別

GATES 用 Audience の GitHub OIDC JWT の署名・Issuer・Audience・時間条件・必須 Claim を検証し、不変 Owner ID / Repository ID で呼び出し元を識別する。これで特定される呼び出し元を **Caller** と呼ぶ。Repository ID は監査のための Caller 識別に用い、認可には用いない。ワークフロー実行 ID と試行番号、およびワークフロー起動属性（起動したイベント、ref、ワークフロー、environment、行為者など）は、監査のためだけに Caller に添え、認可には用いない。ワークフロー起動属性は JWT に含まれる範囲で読み取り、欠落を拒否の理由にしない。

#### FR-AUTHN-003 不正な JWT と許可されない呼び出し元の拒否

許可されない呼び出し元、JWT の欠落・改ざん・期限切れ・Audience 不一致・不正な主体・異なるオーナーを拒否する。

#### FR-AUTHN-004 Caller の根拠としての自己申告の不採用

要求者の自己申告を Caller の根拠にしない。

#### FR-AUTHN-005 不変 Owner ID で呼び出し元を特定できる JWT に限る IAM Role の引き受け

GATES の呼び出しに使う IAM Role の引き受けでは、呼び出し元を不変 Owner ID で特定できる GitHub OIDC JWT だけを受け付け、名前だけで特定する JWT を拒否する。

#### FR-AUTHN-006 検証に使う Claim の一意な解釈

GitHub OIDC JWT の検証に使う Claim は、GATES の検証で Claim を読み取るどの処理でも、名前と値を同じ一つの解釈でしか読み取れない場合に限り受理する。次の JWT は、ほかの条件を満たしていても拒否する。

- 標準仕様では別の Claim だが、GATES の検証で Claim を読み取る処理によっては、検証に使う Claim と同じ名前に読める Claim を含む JWT
- 時刻の Claim の値を、GATES の検証で Claim を読み取る処理によって異なる値に読める JWT

#### FR-AUTHN-007 署名の検証での JOSE Header の拡張と検証鍵の扱い

GitHub OIDC JWT の署名の検証では、GATES が対応しない JOSE Header の拡張を含む JWT は、その拡張を無視せずに拒否する。検証鍵には、署名の検証の用途と署名方式に合い、署名方式が求める強度を満たす鍵だけを使う。

### 5.2 Authorization

#### FR-AUTHZ-001 許可オーナーの Owner ID による認可

検証済み JWT の不変 Owner ID が、許可オーナーの不変 Owner ID と一致することを確認する。オーナー名の一致では代用しない。

#### FR-AUTHZ-002 同一 Installation に含まれるリポジトリへの発行の限定

許可オーナーに属し、GATES で利用する GitHub App の同一 Installation に含まれるリポジトリだけに発行する。含まれない対象を一つでも含む要求にはトークンを返さない。発行前に確認するか発行結果で検出するかは設計に委ねる。発行結果で検出し、トークン文字列を取得できた場合は失効を試みる。

#### FR-AUTHZ-003 権限の `contents:read` への限定と、要求で明示したリポジトリへの対象の限定

現在のスコープでは `contents:read` のみ許可し、その他の権限を拒否する。対象は要求で明示したリポジトリ（GitHub の `repository_selection` でいう `selected`）に限り、全リポジトリ（`all`）を対象にする手段を利用者に与えない。

#### FR-AUTHZ-004 利用者の入力による Caller、Installation ID、認可の根拠、API Host の指定の禁止

利用者の入力から Caller、Installation ID、認可の根拠や任意の API Host を指定できないようにする。

#### FR-AUTHZ-005 要求したリポジトリのオーナーと、GitHub API が返す実効対象の検証

要求されたリポジトリ名のオーナーと、GitHub API が返す実効対象を検証し、要求外のリポジトリへ発行しない。

#### FR-AUTHZ-006 複数対象の要求の全件成立

複数対象の要求は全件が成立する場合のみ許可し、一部成功や要求範囲の自動的な縮小を行わない。

#### FR-AUTHZ-007 利用者による対象リポジトリと権限の明示

利用者に対象リポジトリと権限の明示を要求し、欠落・未知値を広い既定権限に読み替えない。

### 5.3 Token Issuance

#### FR-ISSUANCE-001 秘密鍵の平文を取得しない GitHub App JWT の生成

秘密鍵の平文を取得せずに GitHub App JWT を生成する。

#### FR-ISSUANCE-002 Installation の範囲を上限とする短命なトークンの取得

GitHub App Installation に付与された対象・権限を上限として、GitHub API から短命な Installation Access Token を取得する。有効期間は GitHub の仕様に従い、GATES で延長しない。

#### FR-ISSUANCE-003 発行 API の呼び出しでの対象と権限の明示

**最小権限** を満たすよう、発行 API の呼び出しに対象リポジトリと権限を明示し、要求した対象・権限に限定する。全リポジトリ・全権限の暗黙継承を禁止する。GitHub が要求とは別に付与する権限の許容条件は FR-ISSUANCE-006 に従う。

#### FR-ISSUANCE-004 成功応答の内容

成功応答には Installation Access Token、有効期限、対象リポジトリと実効権限、相関 ID を返す。秘密鍵・内部 GitHub App JWT は返さない。

#### FR-ISSUANCE-005 GitHub App Private Key の更新

新しい GitHub App Private Key へ更新でき、更新後も発行が継続する。

#### FR-ISSUANCE-006 発行応答の照合と、照合に失敗したときのトークンの失効

GitHub API の発行応答で対象一覧・権限の欠落・不整合、要求範囲を超える対象・権限（現在許容する付随権限 `metadata:read` を除く）、不変 Owner ID の不一致を検出した場合は、トークンを返さない。応答から空でないトークン文字列を取得できた場合は失効 API の呼び出しを試み、その成否を監査する。GitHub 側の失効成功および通信断等で取得できなかったトークンの不存在は保証しない。

#### FR-ISSUANCE-007 Docker Action によるトークンの失効

Docker Action は、受け取った Installation Access Token の失効を試み、その成否をワークフローのログに出す。後続ステップへ渡したトークンはジョブの終了時に、受け取った後、後続ステップへ渡す前に失敗した場合はその時点で失効を試みる。ジョブがキャンセルされた場合も、受け取ったトークンの失効をこのどちらかで試みるが、その成否がワークフローのログに残ることは保証しない。失効はベストエフォートとし、失効の失敗でジョブを失敗させない。トークンを受け取る前に失敗した場合は失効を試みない。利用者が失効を止める手段は設けない。

### 5.4 Usability and Operations

#### FR-OPS-001 Docker Action による取得手順の隠蔽

Docker Action が AWS の認証情報の取得、認証、トークン取得の手順を利用者から隠蔽する。利用者に求める作業は、責任分界（§3）に定めたものだけとする。

#### FR-OPS-002 失敗の種別を区別したエラー

入力不正、認証失敗、GATES による認可拒否、Installation 利用不可（発行に使える Installation を得られない）、原因が未確定の GitHub 発行拒否、発行結果の照合失敗、外部 API・KMS の一時的な障害を区別したエラーを返す。Issuer が公開する検証鍵の集合を取得できない障害と、その集合に使用できる鍵が一つもない障害は、JWT の不正による認証失敗と区別する。

#### FR-OPS-003 資格情報を含まない監査記録

認証済み Caller（Owner ID と Repository ID、ワークフロー実行 ID と試行番号、ワークフロー起動属性）、要求対象、要求権限、結果、相関 ID を資格情報を含めず監査可能にする。ワークフロー起動属性は JWT に含まれる範囲で記録する。署名検証を通った GATES 用 OIDC JWT の Claims Set も、Claim の検証で認証に失敗した場合を含めて調査用に記録し、認証と認可には使わない。署名を検証する前の Payload と JWT の文字列は記録しない。相関 ID は成功・失敗のどちらの応答にも含め、呼び出し元が監査記録と突き合わせられるようにする。

#### FR-OPS-004 Terraform による AWS リソースの管理

AWS リソースの設定と変更を Terraform で管理する。秘密鍵素材の取得・変換・インポートと、IAM OIDC Provider の用意は Terraform 管理外とする。

#### FR-OPS-005 鍵の切り替えでの KMS Key の意図しない置換と削除の防止

新しい鍵への切り替えでは KMS Key の意図しない置換・削除を防ぐ。

#### FR-OPS-006 デプロイ時の設定による許可オーナー、GitHub App、KMS Key の固定

許可オーナー（不変 Owner ID で指定）、GitHub App、署名に用いる KMS Key は、GATES 運用者が管理するデプロイ時の設定として固定する。Installation は、固定した GitHub App と許可オーナーから一意に導出する。これらは実行時に外部入力から変更できない。

#### FR-OPS-007 IAM で許可された主体に限る呼び出し口

GATES の呼び出し口は、AWS IAM で許可された主体だけが呼び出せるものとし、認証なしでインターネットに公開された呼び出し口を設けない。IAM で許可する主体は、FR-AUTHN-005 の不変 Owner ID で特定した許可オーナーの GitHub Actions ワークフローに限る。許可されていない主体の呼び出しは、関数が実行される前に拒否される。

## 6. Quality and Security Requirements

性能と可用性の数値目標は、現時点では定めません。

### QR-001 秘密鍵保護

GitHub Secrets、Actions Runner、Lambda のプロセスに Private Key 平文を置かない。鍵インポートは別の運用境界とする。

### QR-002 最小権限

利用者が明示した対象・権限に限定し、Installation の広い権限をそのまま付与しない。GitHub が要求とは別に付与する権限は FR-ISSUANCE-006 の許容条件に従う。

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
