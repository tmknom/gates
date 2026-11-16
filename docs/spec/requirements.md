# Requirements

## 1. Purpose

GATES は、GitHub Actions のワークフローに、GitHub App Private Key を渡すことなく、対象リポジトリと権限を限定した短命な GitHub App Installation Access Token を発行します。
トークンを要求できるのは、許可された単一の **オーナー**（リポジトリを所有する GitHub の Organization またはユーザーアカウント）配下のワークフローであり、対象にできるのは、GitHub App の **Installation**（GitHub App をオーナーにインストールした単位）に含まれるリポジトリです。
ワークフローは、GATES が提供する **利用者接点**（ワークフローから GATES を呼び出す Action）を通じてトークンを要求します。
GATES は AWS Lambda 上で動き、GitHub App Private Key による署名を AWS KMS に委ねます。
GATES の存在理由、解決する問題、設計上の判断原則は [Constitution](../CONSTITUTION.md) で定めます。
本ドキュメントは、そのうち現在のスコープで GATES が満たす要求を定めます。

## 2. Stakeholders and Responsibilities

GATES は、次の責任を各主体が果たすことを前提に要求を満たします。

| 主体 | 責任 |
|---|---|
| ワークフロー利用者 | 利用者接点を呼び出すジョブで `permissions: id-token: write` を宣言し、利用者接点に対象リポジトリ、権限、GATES 運用者が用意した接続情報を渡す。トークンを取得するジョブで、信頼できないコード（Pull Request のコード、ほかのワークフローの成果物など）を実行しない。取得したトークンを、取得したジョブの外（ほかのジョブ、成果物、ログ）へ渡さない。セルフホストの Runner で実行する場合は、利用者接点が必要とする実行環境と通信先を用意する。 |
| GitHub App / オーナーの管理者 | 許可オーナーのどのワークフローから GATES を通じて読み取られてもよいリポジトリだけを、Installation の対象に含める。オーナー配下のリポジトリの OIDC JWT に不変 Owner ID を含める設定を行う。鍵の更新後に GitHub App の旧 Private Key を失効させる。 |
| GATES 運用者 | 許可オーナー、GitHub App、署名に用いる KMS Key をデプロイ時の設定として管理する。GitHub App Private Key を KMS にインポートし、更新する。GATES の呼び出しに使う IAM Role を引き受けられる主体の条件（信頼ポリシー）を管理する。利用者接点を提供し、その入出力を維持する。利用者接点が GATES を呼び出すための接続情報を、ワークフローから参照できるよう GitHub 側に用意する。 |

## 3. Functional Requirements

### 3.1 Authentication

#### FR-AUTHN-002 GitHub OIDC JWT の検証と Caller の識別

GATES 用 Audience の GitHub OIDC JWT の署名・Issuer・Audience・時間条件・必須 Claim を検証し、不変 Owner ID / Repository ID で呼び出し元を識別する。これで特定される呼び出し元を **Caller** と呼ぶ。Caller の Repository ID と、Caller に添えるワークフロー実行 ID と試行番号、ワークフロー起動属性（起動したイベント、ref、ワークフロー、environment、行為者など）は、監査のためだけに用い、認可には用いない。ワークフロー起動属性は JWT に含まれる範囲で読み取り、欠落を拒否の理由にしない。

#### FR-AUTHN-003 不正な JWT の拒否

JWT の欠落・改ざん・期限切れ・Audience 不一致・不正な主体を拒否する。

#### FR-AUTHN-005 許可オーナーの不変 Owner ID で呼び出し元を特定できる JWT に限る IAM Role の引き受け

GATES の呼び出しに使う IAM Role の引き受けでは、`sub` の先頭が呼び出し元のオーナーを許可オーナーの不変 Owner ID で特定する GitHub OIDC JWT だけを受け付ける。`sub` が呼び出し元を名前だけで特定する JWT、`sub` のカスタマイズにより先頭で不変 Owner ID を示さなくなった JWT、`sub` の先頭が別のオーナーの不変 Owner ID を示す JWT は拒否する。

#### FR-AUTHN-006 検証に使う Claim の一意な解釈

GitHub OIDC JWT の検証に使う Claim は、GATES の検証で Claim を読み取るどの処理でも、名前と値を同じ一つの解釈でしか読み取れない場合に限り受理する。次の JWT は、ほかの条件を満たしていても拒否する。

- 標準仕様では別の Claim だが、GATES の検証で Claim を読み取る処理によっては、検証に使う Claim と同じ名前に読める Claim を含む JWT
- 時刻の Claim の値を、GATES の検証で Claim を読み取る処理によって異なる値に読める JWT

#### FR-AUTHN-007 署名の検証での JOSE Header の拡張と検証鍵の扱い

GitHub OIDC JWT の署名の検証では、GATES が対応しない JOSE Header の拡張を含む JWT は、その拡張を無視せずに拒否する。検証鍵には、署名の検証の用途と署名方式に合い、署名方式が求める強度を満たす鍵だけを使う。

### 3.2 Authorization

#### FR-AUTHZ-001 許可オーナーの Owner ID による認可

検証済み JWT の不変 Owner ID が、許可オーナーの不変 Owner ID と一致することを確認し、一致しない呼び出し元を拒否する。オーナー名の一致では代用しない。

#### FR-AUTHZ-002 同一 Installation に含まれるリポジトリへの発行の限定

許可オーナーに属し、GATES で利用する GitHub App の同一 Installation に含まれるリポジトリだけに発行する。含まれない対象を一つでも含む要求にはトークンを返さない。Installation に含まれることは、発行 API を呼ぶ前に確かめても、発行結果の照合（FR-ISSUANCE-006）で確かめてもよい。

#### FR-AUTHZ-003 権限の `contents:read` への限定

現在のスコープでは `contents:read` のみ許可し、その他の権限を拒否する。

#### FR-AUTHZ-004 利用者の入力による Caller、Installation ID、認可の根拠、API Host の指定の禁止

利用者の入力から Caller、Installation ID、認可の根拠や任意の API Host を指定できないようにする。

#### FR-AUTHZ-006 複数対象の要求の全件成立

複数対象の要求は全件が成立する場合のみ許可し、一部成功や要求範囲の自動的な縮小を行わない。

#### FR-AUTHZ-007 利用者による対象リポジトリと権限の明示

利用者に対象リポジトリと権限の明示を要求し、欠落・未知値を広い既定値に読み替えない。対象は要求で明示したリポジトリ（GitHub の `repository_selection` でいう `selected`）に限り、全リポジトリ（`all`）を対象にする手段を利用者に与えない。

### 3.3 Token Issuance

#### FR-ISSUANCE-001 秘密鍵の平文を取得しない GitHub App JWT の生成

秘密鍵の平文を取得せずに GitHub App JWT を生成する。

#### FR-ISSUANCE-002 認可された要求に限る短命なトークンの取得

認可された要求に対してだけ、GitHub API から短命な Installation Access Token を取得する。有効期間は GitHub が定めたものとし、GATES で延長しない。

#### FR-ISSUANCE-003 発行 API の呼び出しでの対象と権限の明示

発行 API の呼び出しに、要求した対象リポジトリと権限を明示し、Installation の全リポジトリ・全権限を暗黙に引き継がせない。GitHub が要求とは別に付与する権限の許容条件は FR-ISSUANCE-006 に従う。

#### FR-ISSUANCE-004 成功応答の内容

成功応答には Installation Access Token、有効期限、対象リポジトリと実効権限、相関 ID を返す。秘密鍵・内部 GitHub App JWT は返さない。

#### FR-ISSUANCE-006 発行応答の照合と、照合に失敗したときのトークンの失効

GitHub API の発行応答で対象一覧・権限の欠落・不整合、要求範囲を超える対象・権限（現在許容する付随権限 `metadata:read` を除く）、不変 Owner ID の不一致を検出した場合は、トークンを返さない。応答から空でないトークン文字列を取得できた場合は失効 API の呼び出しを試み、その成否を監査する。GitHub 側の失効成功および通信断等で取得できなかったトークンの不存在は保証しない。

#### FR-ISSUANCE-007 不要になったトークンの失効

利用者接点は、受け取った Installation Access Token が不要になった時点で失効を試み、その成否をワークフローのログに出す。トークンはジョブをまたいで使わせないため、後続ステップへ渡したトークンはジョブの終了時に、後続ステップへ渡す前に失敗した場合はその時点で失効を試みる。ジョブがキャンセルされた場合も失効を試みるが、その成否がログに残ることは保証しない。失効はベストエフォートとし、失効の失敗でジョブを失敗させない。利用者が失効を止める手段は設けない。

### 3.4 Usability and Operations

#### FR-OPS-002 失敗の種別を区別したエラー

入力不正、認証失敗、GATES による認可拒否、Installation 利用不可（発行に使える Installation を得られない）、原因が未確定の GitHub 発行拒否、発行結果の照合失敗、外部 API・KMS の一時的な障害を区別したエラーを返す。Issuer が公開する検証鍵の集合を取得できない障害と、その集合に使用できる鍵が一つもない障害は、JWT の不正による認証失敗と区別する。

#### FR-OPS-003 資格情報を含まない監査記録

認証済み Caller（Owner ID と Repository ID、ワークフロー実行 ID と試行番号、ワークフロー起動属性）、要求対象、要求権限、結果、相関 ID を資格情報を含めず監査可能にする。署名検証を通った GATES 用 OIDC JWT の Claims Set も、Claim の検証で認証に失敗した場合を含めて調査用に記録し、認証と認可には使わない。署名を検証する前の Payload と JWT の文字列は記録しない。相関 ID は成功・失敗のどちらの応答にも含め、呼び出し元が監査記録と突き合わせられるようにする。

#### FR-OPS-006 デプロイ時の設定による許可オーナー、GitHub App、KMS Key の固定

許可オーナー（不変 Owner ID で指定）、GitHub App、署名に用いる KMS Key は、GATES 運用者が管理するデプロイ時の設定として固定する。Installation は、固定した GitHub App と許可オーナーから一意に導出する。これらは実行時に外部入力から変更できない。

#### FR-OPS-007 IAM で許可された主体に限る呼び出し口

GATES の呼び出し口は、AWS IAM で許可された主体だけが呼び出せるものとし、認証なしでインターネットに公開された呼び出し口を設けない。呼び出しを許可する主体は、FR-AUTHN-005 の条件で引き受ける IAM Role に限る。許可されていない主体の呼び出しは、関数が実行される前に拒否される。

#### FR-OPS-008 発行を止めない署名鍵の切り替え

GATES 運用者は、署名に用いる GitHub App Private Key を新しい鍵へ切り替えられる。切り替えの間も発行を止めず、切り替えの後は、GitHub 側で旧鍵を失効させても発行を続けられる。

#### FR-OPS-009 利用者接点の提供

GATES は利用者接点を提供する。利用者は、対象リポジトリ、権限、接続情報を利用者接点に渡すだけでトークンを取得でき、AWS の認証情報の取得や GATES の呼び出しの手順を扱わない。利用者に求める作業は §2 に定めたものだけとする。利用者接点の入出力は、GATES の内部の実装を変えても利用者のワークフローを変えずに済む境界として保つ。

## 4. Quality and Security Requirements

性能と可用性の数値目標は、現時点では定めません。

### QR-003 Fail Closed

JWT 検証、Installation の解決、認可、署名、発行結果の照合が成立しなければ、利用者に Installation Access Token を返却しない。発行 API の応答を取得できない場合、GitHub 側でトークンが発行されたかは確定できず、残留トークンの不存在までは保証しない。

### QR-004 秘密情報保護

秘密鍵、OIDC JWT、GitHub App JWT、Installation Access Token、AWS 一時認証情報をログ・エラー本文に記録しない。

### QR-008 秘密鍵の保護

GitHub App Private Key の平文を、GitHub Secrets、Actions Runner、Lambda のプロセスに置かない。秘密鍵素材を、Git、インフラの構成管理の状態と計画、CI の成果物に保存しない。鍵素材の KMS へのインポートは、インフラの構成管理とは別の運用で行う。インポート後は、インポート前の秘密鍵のコピーを、インポートを実施した運用者の手元から削除する。Lambda は鍵を署名にのみ使え、鍵素材のインポート・置換・削除や、鍵の管理設定の変更はできない。
