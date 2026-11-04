# Security Design

## 1. Security Model

本システムでは、AWS IAM による Lambda Invoke の入口制御と、Lambda 内で検証した GitHub OIDC JWT に基づくトークン発行認可を分離します。GitHub App Private Key は AWS KMS に置き、Lambda は署名操作を KMS に依頼します。返却する Installation Access Token は、要求されたリポジトリと `contents:read`（許可された付随権限を除く）に限定します。

コンポーネントの全体構成、JWT の生成元、正常系の処理順序は [System Architecture](architecture.md) に記述します。本ドキュメントは各境界が保証する内容と、認可・資格情報保護・拒否条件を定義します。概念の定義は [Domain Model](domain.md) を正本とします。

## 2. Trust Boundaries

| 境界 | 保証すること | 保証しないこと |
|---|---|---|
| AWS IAM / STS | 許可オーナーの不変 Owner ID を `sub` に含む GitHub OIDC JWT からだけ AWS 一時認証情報を発行し、特定 Lambda の Invoke のみ許可する | ワークフロー・リポジトリ単位のトークン発行認可、資格情報の現在の利用者の特定 |
| OIDC Verifier | GitHub JWT の真正性、Issuer、Audience、期限、不変 Owner ID と Repository ID を検証する | JWT と AWS 一時認証情報が同じジョブで取得されたことの証明、トークン発行権限そのもの |
| Authorization | 許可オーナーの Owner ID、取得した Installation の `account.id`、固定権限、明示された要求を照合する | アクセス先ごとの Caller の同意確認、対象リポジトリが Installation に含まれることの事前確認 |
| AWS KMS | 非抽出可能な秘密鍵による RSA 署名を実行する | 署名対象 JWT の内容に対する GATES の認可判断 |
| GitHub API | Installation と App の権限上限に従い短命トークンを発行する | GATES 側のトークン発行認可 |

### 2.1 AWS IAM / STS

IAM Role の Trust Policy は、GitHub OIDC Provider を信頼先とし、`aud` と `sub` の条件で引き受けを制限します。
AWS 用 OIDC JWT の Claim に課す条件と、この入口制御が保証しない範囲は [JWT Design](jwt.md#3-aws-用-oidc-jwt) に定めます。

- オーナー配下での共通利用を前提とし、`sub` の後半部分（`ref`、`pull_request`、`environment` など）による追加制限は設けない。
- 引き受けた IAM Role の権限は対象 Lambda への `lambda:InvokeFunction` のみとし、KMS `Sign`、IAM 管理、鍵インポートを許可しない。
- 許可オーナーのワークフローは共通の一つの IAM Role を引き受ける。

オーナー側で既存リポジトリの OIDC JWT に不変 ID を含める設定が済んでいることを前提とします。この設定は GitHub 側の操作であり、GATES は実施しません（[Requirements](../spec/requirements.md) §2.2）。設定前のリポジトリは旧形式の `sub` を持つため、IAM Role を引き受けられません。

IAM は認証情報の由来となるオーナーに対する入口制限であるため、IAM 認証情報の窃取・再利用により、元のジョブ以外から Lambda Invoke が行われる可能性は残ります。

### 2.2 GATES 用 OIDC JWT

AWS 用 OIDC JWT とは別に、GATES 用 Audience の OIDC JWT を取得します。
Lambda はこの JWT を検証し、Caller を確定します。
署名と検証鍵、Issuer、Audience、時間条件、必須 Claim、`sub` と個別の ID の Claim の整合の受理条件は [JWT Design](jwt.md) に定めます。
検証鍵は固定した Issuer の JWKS からだけ得ます。
JWT の発行者と署名検証鍵を結び付け、JWT や要求の内容で鍵の取得先を変えられないようにするためです。

JWT が受理条件を満たしても、発行要求を認可したことにはなりません。
発行の可否は、§3 の認可条件で別に判定します。

AWS 一時認証情報と GATES 用 OIDC JWT が同じジョブで取得されたことを、暗号学的に検証しません。AWS IAM による Invoke の許可と、GATES 用 OIDC JWT に基づく発行認可は独立した境界です。トークンの発行可否は GATES 用 OIDC JWT と固定された許可条件から判断し、Composite Action 側の自己申告による照合を認可根拠としません。

## 3. Authorization

### 3.1 Authorization Rule

リポジトリへの発行は、次の条件がすべて成立した場合だけ許可します。
いずれかが成立しなければ、認可拒否として扱います。

1. 検証済み Caller の `repository_owner_id` が許可オーナーの不変 Owner ID と一致する。オーナー名の一致では代用しない。
2. 要求した各 `owner/repo` のオーナー名が固定された許可オーナー名と一致する。
3. 要求の Permissions が **`contents:read` のみ** である。

Caller の `repository_id` とワークフロー起動属性は監査のための識別に用い、認可条件には含めません。

条件 3 の Permissions は、形式が正しければ JWT 検証後に認可条件として判定し、許可外なら認可拒否として扱います。
形式検証で先に落とさないのは、`contents:write` のような許可外の要求を、認証済み Caller とともに監査記録に残すためです。
形式検証が入力不正にするのは、欠落や空の Permissions のように、要求した権限を読み取れない形に限ります。

要求には、全リポジトリを対象にする項目を設けません。
対象は要求で明示したリポジトリだけであり、`selected` しか許可しない値を利用者に指定させても、許可条件を一つ増やすだけになるためです。

認可条件が成立した後、GitHub App JWT で許可オーナーの Installation を取得し、発行に使えるかを次の条件で確認します。

- Installation の `account.id` が許可オーナーの Owner ID と一致する。
- Installation が停止していない（`suspended_at` が `null` である）。`suspended_at` のキーがない、または `null` でも時刻でもない応答は、停止していないと読まず、Installation を作らない。

どちらかが成立しなければ発行しません。
この拒否は利用者の要求に起因せず、運用者かオーナーの管理者が GitHub 側か固定設定を直す必要があるため、認可拒否と区別し、Installation 利用不可として返します。
停止中の Installation に対する発行 API の応答は公式文書で確認できていないため、GitHub の振る舞いに Fail Closed を預けず、取得した時点で閉じます。

これらの確認の後、Installation ID、要求した完全名、許可権限を含む発行許可を生成します。Installation に対象リポジトリが含まれることと、発行結果が発行許可に一致することは、GitHub API の応答を受けてから確認します。一部だけで発行することや権限を暗黙に広げることは禁止します。

### 3.2 Installation Scope as Authorization Boundary

Installation の GitHub 側の仕様は [GitHub Constraints](../spec/github-constraints.md#5-installation) を参照します。

許可オーナーの有効なワークフローは、呼び出し元リポジトリ／チーム別の認可を経ず、GATES が許可する権限の範囲で GitHub App の Installation 対象リポジトリに横断的なトークン発行を要求できます。この権限は GitHub App / Installation 自体の権限上限にも拘束されます。GATES が許可するのは `contents:read` のみです。

GitHub App のインストール対象が読み取り可能範囲の上限となります。`Only select repositories` により対象を限定できます。`All repositories` を使用すると、そのオーナーにインストールされた App の対象リポジトリ全体が読み取り可能範囲に含まれます。インストール対象の変更は認可境界の変更として扱います。

許可オーナーが Organization の場合、この変更は Organization オーナーだけの操作ではありません。
既定では、リポジトリ管理者が自身の管理するリポジトリをインストール済みの App の対象に追加でき、`installation_repositories:write` を持つ呼び出し元も、そのリポジトリを Installation に追加または削除できます。
追加されたリポジトリは、GATES と Installation が許可する権限の範囲で、許可オーナーの有効なワークフローから横断的にアクセスされ得ます。許可する権限は `contents:read` のため、非公開コードや設定、誤って格納された秘密情報の読み取りが主な懸念です。
このため、Organization の許可オーナーには、リポジトリ管理者によるインストールを Organization オーナーが制限していることと、`installation_repositories:write` の付与先を管理していることを、運用の前提として求めます。
GATES はこの設定を実行時に確認しません。

ここでの上限は、Installation Access Token の対象に指定できる範囲を指します。
「インストール対象の外にある公開リポジトリは読めない」を意味しません。
GitHub App が公開リポジトリを読み取れることとトークンの対象に指定できることは別であり、選択対象外の公開リポジトリに対するトークンの振る舞いは確定していないためです。

GATES は、デプロイ時に固定した GitHub App と許可オーナーの組から Installation が一意に決まるものとして扱います。
これは、オーナーを指定して単一の Installation を返すエンドポイントがあることからの推論であり、GitHub が一意性を明示的に保証したものとは扱いません。
Lambda は Installation ID を設定として持たず、固定したオーナーの Installation を GitHub API から取得し、`account.id` の一致で確認します。この導出は [Requirements](../spec/requirements.md) FR-024 に従います。Installation ID を設定値として持たないため、アプリの再インストールで ID が変わっても設定変更は不要であり、実行時の入力から Installation を指定する余地もありません。オーナーの名前変更やアプリの再インストールが起きても、固定した不変 Owner ID と異なる Installation へ発行しません。

### 3.3 Permission Boundary

Lambda が許可する権限は `contents:read` に固定します。要求された Permissions が許可条件と異なる場合は拒否します。GitHub App / Installation の付与権限が GATES の許可範囲より広くても、発行するトークンの権限は拡大しません。付随する `metadata:read` のみ、§3.4 の条件で照合時に許容します。

### 3.4 Issuance Result Verification

- GitHub API に発行を要求するときは、発行許可の対象と Permissions を必ず指定する。
- 発行応答の対象リポジトリと実効権限を発行許可に照合する。Owner ID、Repository ID の有効性・一意性、対象の全件一致を確認し、必要な情報の欠落、対象外のリポジトリ、過大な権限があればトークンを返却しない。この拒否は GitHub API の通信障害とは別のエラーとして返し、監査記録に Caller とともに残す。
- GitHub の発行応答からトークン文字列を取得でき、照合に失敗した場合は、利用者へ返却せず失効 API の呼び出しを試みる。失効はベストエフォートとし、試行の成否を監査記録に残す。API 応答が失われた場合はトークンの発行有無が不明となり得る。
- 要求にかかわらず GitHub が付与する `metadata:read` だけは、応答に含まれていても許容する。付与の条件は未確認であり（[GitHub Constraints](../spec/github-constraints.md#62-発行応答permission)）、含まれなくても拒否しない。
- 発行応答の `repository_selection` が `selected` であることを確認する。`all` は Installation の全リポジトリを対象にしたことを意味し、要求外の対象を含むため返却しない。
- 応答の Repository ID は発行結果の確認・監査に利用し、要求時に事前検証した ID と同一であることを証明するものではない。

対象の全件一致を照合するため、[Requirements](../spec/requirements.md) FR-007 のうち範囲外の対象を含む要求にトークンを返さないことは、GitHub が範囲外の対象を拒否するかどうかに依存しません。
GitHub が範囲外の対象を黙って除いて発行した場合は、応答の対象と発行許可の不一致を検出し、トークンを返さず、取得したトークンの失効を試みます。
発行 API が呼び出しを `422` で拒否した場合は GitHub 発行拒否、発行応答を取得して照合が失敗した場合は照合失敗とします。どちらも、発行前の GATES の認可条件による認可拒否とは区別します。
GitHub 側での失効成功や、応答喪失で取得できなかったトークンの不存在は保証せず、GitHub 側で有効なまま残る可能性があります。

## 4. Secret and Token Lifecycle

KMS Key の構成・IAM 権限は Terraform で管理します。GitHub App Private Key の取得・変換・鍵素材インポートは独立した運用で行い、Terraform の State / Plan と Go アプリケーションには鍵素材を渡しません。

| 資格情報 | 保存・利用の境界 |
|---|---|
| GitHub App Private Key | KMS インポート前は運用者が別の運用境界で扱う。Lambda と Runner には渡さない。インポート後、インポート前のコピーは運用者の手元から削除する。KMS にインポートした鍵素材は取り出せないため、KMS Key を失った場合は GitHub 側で新しい秘密鍵を発行して切り替える |
| AWS KMS Key | インポート済みの非対称署名鍵。Lambda 実行 Role に当該キーの `kms:Sign` のみ許可。鍵更新時は新 KMS Key を用意 |
| GitHub App JWT | KMS 署名で Lambda が短時間生成し、GitHub API 呼び出しにのみ使用。外部返却・ログ出力禁止 |
| Installation Access Token | 要求・認可された対象と `contents:read`（付随する `metadata:read` を許容）に限定して Runner へ返す。短命でも窃取リスクは残る。照合に失敗したものは返さず、文字列を取得できた場合に失効を試みる |
| AWS 一時認証情報・GATES 用 OIDC JWT | Runner で短時間だけ保持し、ログやデバッグ出力に含めない |

## 5. Failure, Audit, and Security Risks

- 入力が不正、または認証に失敗したら、GitHub API や KMS のトークン発行処理に進まない。Installation の解決や署名の失敗、発行結果の不一致は Fail Closed とする。
- 監査記録には相関 ID、時刻、処理結果（成功またはエラーコード）、拒否の理由に加え、認証済み Caller の Owner ID と Repository ID、ワークフロー実行 ID と試行番号、ワークフロー起動属性、要求対象（`owner/repo`）、要求権限、取得した Installation の Installation ID と `account.id`、照合に失敗したトークンの失効の成否を記録する。ワークフロー実行 ID と試行番号を残すのは、監査記録から GitHub 上のワークフロー実行にたどり着けるようにするためで、認可には使わない。認証前の失敗では Caller が確定していないため、未検証の値を認証済み主体として記録しない。秘密鍵、OIDC JWT、GitHub App JWT、Installation Access Token、AWS 一時認証情報は記録しない。
- ワークフロー起動属性（`actor`、`event_name`、`ref`、`workflow_ref`、`environment` など）を残すのは、後述の `pull_request_target` の悪用のように、どのイベントと ref でトークンを得たかを調べる場面に備えるためである。GitHub 側の記録は保持期間を過ぎたり削除されたりし得る。ワークフロー起動属性はベストエフォートで記録し、Claim が欠落していても拒否しない。IAM Role の引き受けと Lambda の呼び出しは AWS 側の記録（CloudTrail など）が担い、GATES の監査記録はそれを補うアプリケーション層の記録と位置づける。
- 相関 ID は成功応答と失敗応答の両方に、同じ位置で含める。ワークフローのログに残った相関 ID から監査記録へたどれるようにするためである。
- AWS Invoke の成功と Lambda の業務成功を区別し、Composite Action は `FunctionError` と応答の両方を確認する。
- 許可オーナー配下のリポジトリであることは、ワークフロー内のコードの信頼性を保証しない。`pull_request_target` などで信頼できない Pull Request のコードを実行すると、正当な JWT と AWS 認証情報を取得できるジョブの権限を悪用され得る。利用者は資格情報を取得するジョブで信頼できないコードを実行しない。
- JWT、AWS 認証情報、Installation Access Token の漏えい・再利用を完全には防止しない。許可範囲、期限、権限とインストール対象の限定で影響を抑える。
- 保持した Issuer の公開鍵に有効期限を設けず、次のリスクを受容する。Issuer が JWKS から鍵を取り下げても、保持した鍵で検証できない `kid` のある JWT が届いて JWKS の取得に成功するか、実行環境が入れ替わるまで、取り下げた鍵で検証できる JWT を受理し続ける。Lambda は呼び出しが続く関数でも実行環境を数時間ごとに終了するため、受理し続ける期間はその程度に収まる見込みだが、AWS は正確な間隔を公開しておらず、GATES はこの期間の上限を保証しない。取り下げた鍵で新しい JWT を偽造するには、対応する秘密鍵が必要である。一方、取り下げ前に発行された JWT の再提示には秘密鍵が要らず、その JWT の `exp` までは受理され得る。どちらの悪用にも、Lambda の Invoke を許可された IAM 認証情報が別に必要である。期限を設けると、期限切れのたびに取得が発生し、Issuer の障害で正当な JWT まで拒否し得る。この可用性の低下を避ける。Issuer の鍵の漏えいが判明したときは、運用者が関数のコードか設定を更新する。AWS は更新の完了まで呼び出しが更新前のコードと設定を使うと定めており、完了後の呼び出しが保持した鍵を持たない実行環境で動くことは、この定めからの推論である（公式文書に明記はなく、実環境では未確認）。
- Discovery と JWKS の再取得の上限は実行環境ごとであり、関数全体の上限を設けず、次のリスクを受容する。IAM の認可を得た呼び出し元は、手元の鍵で検証できない JWT を送ることで、実行環境ごとに、再取得の上限が許す頻度まで取得を起こせる。署名の検証には鍵が要るため、取得の前に不正な JWT を見分けることはできない。Issuer がレート制限を返すと、鍵を持たない実行環境とローテーション後の鍵を要する JWT は、再試行できる失敗で拒否され続け得る（Issuer のレート制限の有無は未確認）。影響は可用性にとどまり、この場合も GATES はトークンを返さない。実行環境をまたいで上限を共有すると、そのためだけに外部の状態、権限、障害の経路が増えるため採らない。同時に動く実行環境の数は Reserved Concurrency で抑えられるが、実行環境が入れ替わると新しい環境は取得の履歴を持たないため、厳密な上限にはならない。

### 5.1 KMS Signing Key Risks

KMS Key の署名能力は、GitHub App として認証する手段そのものです。
そのため、Key Policy と IAM で抑える範囲と、それでは抑えられずに受容する範囲を分けて定めます。

次のリスクは、設定で抑えます。

| リスク | 抑える手段 |
|---|---|
| 秘密鍵の取り出し | KMS にインポートした鍵素材は取り出せない。Terraform の State / Plan、Git、CI の成果物、Lambda の環境変数に鍵素材を渡さない |
| 実行 Role による鍵の管理 | 実行 Role に許可する KMS の操作を `kms:Sign` だけにする |
| 実行 Role による別の方式での署名 | Key Policy で署名アルゴリズムを `RSASSA_PKCS1_V1_5_SHA_256` に限る |
| 実行 Role による別の鍵での署名 | 実行 Role の IAM ポリシーに他の鍵への許可を含めず、他の鍵の Key Policy に実行 Role を書かない |
| IAM の管理権限を持つ主体による直接の署名 | Key Policy で root に許可する操作を列挙し、`kms:Sign`、`kms:*`、`kms:CreateGrant` を含めない |
| Terraform の操作による鍵の削除と置換 | `prevent_destroy` で該当する Plan を拒否する |
| 予約した削除の実行 | 削除の予約期間を 30 日にし、その間は `kms:CancelKeyDeletion` で取り消せるようにする |
| CI からの権限の変更 | Terraform を運用者の手元から適用し、CI に適用権限を与えない |

これらの設定は「鍵を管理できる主体は署名できない」を意味しません。
保証するのは、実行 Role 以外の主体が署名するには、Key Policy、Lambda、実行 Role の Trust Policy のいずれかの変更か、実行 Role を別の関数に渡す操作が別に必要になるところまでです。

次のリスクは受容します。

- **実行 Role の侵害**：Lambda のコードの脆弱性などで実行 Role の権限を奪われると、`kms:Sign` で不正に署名できる。KMS は秘密鍵の取り出しを防ぐが、署名能力の悪用は防がない
- **`kms:PutKeyPolicy` を持つ主体**：Key Policy を書き換えて、自身に署名を許可できる。`kms:CreateGrant` を一時的に許可して作った Grant は、Key Policy を元に戻しても残る。この Grant を確認して取り消せるように、root に `kms:ListGrants` と `kms:RevokeGrant` を許可する
- **Lambda の更新権限を持つ主体**：関数のコードや設定を変更して、実行 Role の権限で署名させられる。`iam:PassRole` を持つ主体は、実行 Role を別の関数に渡して同じことができる
- **`iam:UpdateAssumeRolePolicy` を持つ主体**：実行 Role の Trust Policy を書き換えて Role を引き受け、署名できる
- **Permissions Boundary を通る別の鍵での署名**：Boundary は同じアカウントとリージョンの KMS Key のうち、鍵素材をインポートした鍵への `RSASSA_PKCS1_V1_5_SHA_256` の `kms:Sign` を許可する。GitHub Actions が引き受ける IAM Role と Lambda 実行 Role のどちらかが、この条件に合う別の鍵で署名できる経路は 2 つある。Role に `kms:Sign` を許可するポリシーが追加され、その鍵の Key Policy が IAM ポリシーへ委任している経路と、その鍵の Key Policy が Role を Principal として直接許可する経路である。後者は Role 側のポリシーが変わらない。GATES の署名鍵は Key Policy が root に `kms:Sign` を許可しないため、前者の経路では署名できない。Boundary を GATES の署名鍵に絞ると Terraform の依存が循環するため、この範囲を受容する
- **適用とインポートを同じ主体が担う構成**：Key Policy は鍵素材のインポートを許可する主体を区別しない。鍵素材のインポートの権限を通常の Terraform 適用から分離することは IAM の権限の割り当てに依存し、Key Policy では保証しない
- **インポート前の秘密鍵のコピー**：運用者の手元のコピーを削除することは手順で定めるだけで、設定では強制できない
- **Terraform の設定の除去による鍵の削除**：`prevent_destroy` はリソースの設定自体を取り除いた場合の削除を防がない。Plan に現れた鍵の削除を運用者が確認して止める
- **KMS の API からの鍵の無効化と削除の予約**：`kms:DisableKey` や `kms:ScheduleKeyDeletion` を IAM で許可された主体は、Terraform を経ずに実行できる。`prevent_destroy` は効かず、Lambda は署名できなくなってトークンを発行できない。影響は可用性にとどまる。鍵の悪用が判明したときに一時的に止める手段でもあるため、Key Policy から外さない

`kms:PutKeyPolicy`、Lambda の更新権限、`iam:UpdateAssumeRolePolicy` によるリスクを受容するのは、これらが Terraform の適用に必要な権限と重なり、Key Policy の記述では取り除けないためです。
`kms:PutKeyPolicy` を Key Policy から外すと Terraform で Key Policy を更新できなくなり、署名を拒否する `Deny` を加えても同じ権限で消せます。
これらの権限を持つ主体は、実行主体の限定と変更レビューで制御します。

不正に署名されると、GATES の認可を経ずに GitHub App JWT を作れるため、§3.1 と §3.3 の制限は効きません。
得られるトークンの上限は、GitHub App の権限と Installation の対象で決まります。
GATES は不正な変更や署名を通知する仕組みを持たないため、悪用に気付くまでの時間は保証できません。
悪用が判明したときは、GitHub 側で Private Key を失効させて止めます。
`kms:DisableKey` による停止は `kms:EnableKey` を持つ主体が元に戻せるため、これだけでは確実に止められません。
`kms:DeleteImportedKeyMaterial` は Key Policy に含めません。
インポート前のコピーを削除するため、鍵素材を消すと同じ鍵には戻せず、悪用を止める目的には GitHub 側の失効で足りるからです。

## 6. Traceability

| 機能 | 対応要求 |
|---|---|
| 入口制御と Caller の識別（§2） | FR-001〜005、FR-025、QR-007 |
| 発行認可（§3.1〜3.3） | FR-006〜012、FR-024、QR-002、QR-007 |
| 署名と発行結果の検証（§3.4、§4） | FR-013〜018、QR-001、QR-003、QR-006 |
| 失敗時の扱いと監査（§5） | FR-020、FR-021、QR-003、QR-004 |
| 署名鍵のリスクの抑制と受容（§5.1） | FR-013、FR-023、QR-001、QR-006 |

受け入れ条件は [Requirements](../spec/requirements.md) を正本とします。

## 7. Sources

- [GitHub Constraints](../spec/github-constraints.md)
- [JWT Design](jwt.md)
- [JWT Constraints](../spec/jwt-constraints.md)
- [GitHub — Securely using pull_request_target](https://docs.github.com/en/actions/reference/security/securely-using-pull_request_target)
- [AWS — IAM and AWS STS condition context keys](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_iam-condition-keys.html)
- [AWS — Lambda Invoke API](https://docs.aws.amazon.com/lambda/latest/api/API_Invoke.html)
- [AWS — Lambda execution environment lifecycle](https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtime-environment.html)
- [AWS — Lambda function states](https://docs.aws.amazon.com/lambda/latest/dg/functions-states.html)
- [AWS — KMS Sign API](https://docs.aws.amazon.com/kms/latest/APIReference/API_Sign.html)
- [AWS — Imported key material](https://docs.aws.amazon.com/kms/latest/developerguide/importing-keys-import-key-material.html)
- [AWS — KMS key policies](https://docs.aws.amazon.com/kms/latest/developerguide/key-policies.html)

## Appendix: Impact of Future Scope

[Future Scope](../spec/future-scope.md) が想定する機能を取り込む場合、本ドキュメントには次の影響が及びます。

- **細粒度の認可**：Caller や対象に応じた条件が認可条件（§3.1）に加わり、横断的な要求を前提とする §3.2 の残留リスクを見直す。監査のためだけに用いている `repository_id` とワークフロー起動属性を認可に使うなら、認可の根拠にできるかを評価する
- **発行可能な権限の拡大**：認可条件 3（§3.1）、権限の境界（§3.3）、発行結果の照合（§3.4）が変わる。読み取り以外の権限を許可すると、§3.2 の懸念は読み取りにとどまらなくなる。Account / Organization に関係する権限は、リポジトリを単位とする認可条件と照合をそのまま一般化しない

スコープを変更するときは、先に本ドキュメントで脅威と緩和策を評価し、その結果を踏まえて [Requirements](../spec/requirements.md) と各設計書を変更します。
