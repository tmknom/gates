# JWT Design

## 1. Scope and Principles

本ドキュメントは、GATES が扱う JWT の **検証・生成に関する契約** を定めます。認証できる条件、Claim の意味と使い方、Claim 間の整合、署名と検証鍵の受理条件を記述します。Go のパッケージ構成、Discovery / JWKS の取得・キャッシュ、エラー応答、IAM Trust Policy の実際の記法、信頼境界と残留リスクはそれぞれのコンポーネント設計の責務です。Caller とその不変条件の概念的な定義は [Domain Model](domain.md) を正本とします。

**文書の状態**：本書の「GATES の受理条件」「GATES の生成条件」は、採用する設計契約です。外部仕様の未確認事項は §7 に整理します。

### 1.1 JWT の用途と検証境界

| JWT | 生成元 | 検証・受信者 | GATES における役割 |
|---|---|---|---|
| AWS 用 OIDC JWT | GitHub Actions OIDC Provider | AWS STS | IAM Role の引き受け。`aud=sts.amazonaws.com` と不変 ID 入り `sub` の接頭部分を制約する |
| GATES 用 OIDC JWT | GitHub Actions OIDC Provider | Lambda の OIDC Verifier | Caller の認証とワークフロー起動属性の監査。GATES 固有の Audience を使用する |
| GitHub App JWT | Lambda が構築し、KMS が署名 | GitHub API | GitHub App として Installation の取得と Installation Access Token 発行を要求する |

AWS 用と GATES 用の OIDC JWT は、同じジョブで Audience を変えて別々に取得します。

### 1.2 根拠と規則の読み方

| 区分 | 意味 |
|---|---|
| 標準仕様 | JWT / JWS / JWK の RFC、OpenID Connect Core / Discovery の **適用対象を特定した** 規範的要求。原文の MUST / SHOULD / MAY の強さを保持する |
| GitHub 公式仕様 | GitHub が文書で説明している発行・設定・Claim の意味。条件付きの規定を一般化しない |
| 公開値・実測値 | 調査記録にある Discovery / JWKS の取得と、Audience を指定しない JWT 6件、URL でない文字列を Audience に指定した JWT 2件の観測。発行条件・将来の仕様を保証しない |
| GATES の設計 | GATES が要求する必須性、型、受理・拒否条件、使用目的。外部仕様にない制約はその旨と理由を示す |
| 未確認 | 文書で仕様を断定できない事項、実測条件では確認できない事項。推測で必須性や形式を補わない |

GATES は、適用対象の標準仕様が要求する検証を省略しません。追加で受理対象を狭くする場合は、どの標準要求に違反しないか、互換性にどのような影響があるかを明記します。「理解しない Claim / Header は無視する」という標準規定も尊重し、無関係な新規 Claim の追加だけを拒否の理由にしません。JSON の名前重複、不正な UTF-8、既知 Claim の解釈の曖昧さは、未知 Claim と区別して §2.38.4 と §5.1 の規則で扱います。

標準仕様の整理は [JWT Constraints](../spec/jwt-constraints.md)、GitHub の仕様と観測は [GitHub Constraints](../spec/github-constraints.md) を正本とします。外部仕様の引用は正確な要約と節へのリンクを用い、全文転記しません。`claims_supported` はサポートする Claim の一覧であり、すべての JWT に全 Claim が存在する保証ではありません。

### 1.3 用語と受理方針

- **必須**：欠落、`null`、型の不一致、個別条件の不一致を拒否する。必須 Claim が成立しなければ Caller を生成しない。
- **任意・検証**：欠落を許す。存在する場合は定めた型・値・時間条件を満たす必要がある。
- **任意・監査**：欠落や型の不一致を許し、正しい型の値だけを監査に転送する。認可の根拠には使用しない。監査する値は構造化形式でエスケープして記録する。
- **未使用**：Claim 自体の存在・値で追加の認可判断をしない。未知 Claim を含む JWT も、共通 JSON / 署名規則には従う。
- **拒否**：GATES 用 OIDC JWT の検証条件を満たさない場合は Caller を確定せず、認証失敗として扱う。鍵の取得や JWKS 全体の異常による失敗は、JWT 自体の検証失敗とは別の障害として扱う。

## 2. GATES 用 OIDC JWT

### 2.1 Claim Summary

以下の Claim は、[GitHub OIDC Discovery](../research/github/oidc-discovery.md) の `claims_supported` に一致します。表は GATES での用途を先に把握するための索引です。正確な受理条件・根拠・例外は各 Claim の節と §2.38 に定めます。「GitHub 差分・観測上の補足」欄の `—` は標準適合性を網羅的に証明した意味ではありません。

| Claim | 外部仕様の要旨 | GATES の条件・用途 | GitHub 差分・観測上の補足 |
|---|---|---|---|
| `sub` | JWT の主体。OIDC では再割当て禁止 | 必須。不変形式を解析し ID Claim と照合 | 旧形式は名前を再利用可能。新旧が混在 |
| `aud` | 受信者。文字列または文字列配列 | 必須。GATES 用 Audience のみ | カスタム Audience を要求可能 |
| `exp` | NumericDate。有効期限 | 必須。期限切れ拒否、猶予なし | 実測では `iat+300` 秒 |
| `iat` | NumericDate。発行時刻 | 必須。未来時刻の上限を検証 | 実測では数値 |
| `iss` | 発行者 | 必須。固定 GitHub Issuer と完全一致 | — |
| `jti` | JWT ID | 未使用。再利用検出を行わない | 実測では文字列 |
| `nbf` | NumericDate。利用開始時刻 | 任意・検証。存在時は時刻を確認 | 実測は `iat-300` 秒。公式例は異なる |
| `ref` | 起動時の Git ref | 任意・監査 | イベントで値が変わる |
| `sha` | コミット SHA | 任意・監査 | Discovery にあるが GitHub の Claim 一覧表に説明なし |
| `repository` | 実行元リポジトリ名 | 未使用 | 名前は再利用・変更可能 |
| `repository_id` | 実行元リポジトリ ID | 必須。`sub` と照合、Caller 識別・監査 | 実測は数字の文字列 |
| `repository_owner` | 実行元のオーナー名 | 未使用 | User を含む説明は公式文書で不十分 |
| `repository_owner_id` | リポジトリ所有者の ID | 必須。`sub` と照合、認可 | 公式は Organization と説明。User でも実測 |
| `enterprise` | Discovery に掲載 | 未使用 | 6件では出現なし。詳細・型は未確認 |
| `enterprise_id` | Discovery に掲載 | 未使用 | 6件では出現なし。詳細・型は未確認 |
| `run_id` | Workflow Run ID | 必須。監査識別 | 実測は数字の文字列 |
| `run_number` | ワークフローの実行回数番号 | 未使用 | 実測は数字の文字列 |
| `run_attempt` | 実行の再試行回数 | 必須。監査識別 | 初回の Claim 値は公式記述だけでは未確定 |
| `actor` | 実行開始者のアカウント | 任意・監査 | 実測は文字列 |
| `actor_id` | 実行開始者の ID | 任意・監査 | 実測は数字の文字列 |
| `workflow` | ワークフローの名前 | 未使用 | `workflow_ref` と区別 |
| `workflow_ref` | ワークフロー定義の参照パス | 任意・監査 | 再利用ワークフローでは呼び出し元を示す |
| `workflow_sha` | ワークフローファイルの SHA | 未使用 | `sha` と異なる場合がある |
| `head_ref` | Pull Request の元ブランチ | 未使用 | PR 以外の実測でも空文字列で存在 |
| `base_ref` | Pull Request の宛先ブランチ | 未使用 | PR 以外の実測でも空文字列で存在 |
| `event_name` | 起動イベント | 任意・監査 | イベント名は認可に使用しない |
| `ref_type` | Git ref の種別 | 未使用 | 実測は文字列 |
| `ref_protected` | Discovery に掲載 | 未使用 | 実測は真偽値ではなく文字列 `"false"` |
| `environment` | ジョブの Environment | 任意・監査 | 使用した実測の1件でのみ存在 |
| `environment_node_id` | Discovery に掲載 | 未使用 | 使用した実測の1件でのみ存在 |
| `job_workflow_ref` | 再利用ワークフローの参照パス | 任意・監査 | 非再利用でも `workflow_ref` と同値で出現 |
| `job_workflow_sha` | 再利用ワークフローの SHA | 未使用 | 非再利用でも実測に存在 |
| `repository_visibility` | リポジトリの公開範囲 | 未使用 | GitHub は `internal` / `private` / `public` と説明 |
| `runner_environment` | ランナーの種別 | 未使用 | GitHub は `github-hosted` / `self-hosted` と説明 |
| `issuer_scope` | Discovery に掲載 | 未使用 | 意味、型、出現条件は未確認 |
| `check_run_id` | 現在のジョブの Check Run ID | 未使用 | 実測は数字の文字列 |

表の必須・任意・未使用は GATES の設計分類であり、GitHub の発行保証ではありません。

この一覧は Discovery の取得時点の掲載名に限ります。Organization / Enterprise の設定に応じて追加される `repo_property_*` や、将来の未知 Claim はこの一覧に含みません。これらの追加 Claim の扱いは §2.38.5 に定めます。

### 2.2 `sub`

**標準仕様**：RFC 7519 §4.1.2 の Subject は Issuer 内で局所的に一意、または全体で一意である必要があります。OpenID Connect Core §2 は ID Token の `sub` を必須、Issuer 内で一意かつ再割当て不可、最大255 ASCII文字、大小文字を区別する文字列と定めます（[RFC 7519 §4.1.2](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.2)、[OIDC Core §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)）。

**GitHub 公式仕様**：旧形式 `repo:OWNER/REPO:...` は名前の再利用によって Subject が再割当てされ得ます。一定の時期より後に作成されたリポジトリ等では `repo:OWNER@OWNER-ID/REPO@REPO-ID:...` の不変形式が導入されています。古いリポジトリでは設定変更まで旧形式が残り、`include_claim_keys` によるカスタマイズで `sub` 全体が変わり得ます。不変形式の `repo` 部分では ID を取り除けないとされていますが、`repo` を含めないカスタムテンプレートの結果は未確認です（[GitHub OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)、[同 — Customizing the token claims](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-token-claims)、[GitHub Constraints §3.2](../spec/github-constraints.md#32-immutable-subject-とカスタマイズ)）。

**実測値**：6件すべてが不変形式であり、末尾は `ref`、`pull_request`、`environment` に応じて異なりました。リポジトリの作成日と不変形式への設定変更履歴は未取得です（[GitHub OIDC JWT](../research/github/oidc-jwt.md)）。

**GATES の受理条件**：

1. `sub` は欠落不可の JSON 文字列。`repo:` で始まり、以後の最初の `/` によってオーナー部とリポジトリ部を分ける。
2. リポジトリ部は最初の `:` までとし、`:` がなければ拒否する。オーナー部・リポジトリ部はそれぞれ非空の `NAME@ID` で、`@` は各部にちょうど1個とする。
3. 両方の ID は §2.38.1 の正の整数の文字列表現に合致する。`repository_owner_id`・`repository_id` との一致は §2.38.2 で判定する。
4. 名前部分は ID の認可根拠とせず、Claim の名前部分と他の名前の一致を認証条件にしない。`:` 以降のコンテキストは解釈・認可に使用しない。
5. 旧形式、`repo` を含まないカスタマイズ形式、末尾の `:` がない形式は拒否する。`repo` だけを指定するカスタマイズに対応しないのは現在の GATES の対象範囲による。
6. `sub` 全体を ASCII 文字のみに限定し、文字数が255を超えたら拒否する。255文字の上限は、OpenID Connect Core §2 の ID Token `sub` の最大255 ASCII文字という規定に従う。ASCII 以外を拒否することは GATES の追加制約である。長い ref / Environment や Unicode を含むコンテキストにより GitHub がこの条件を満たさない JWT を発行し得るかは未確認であり、そのような JWT を拒否する互換性上の影響を受け入れる（§7）。

**理由**：`sub` の名前部分だけを信頼すると、名前の再利用で別の主体を許す危険があります。不変 ID と個別 Claim の両方を確認し、主体の取り違えを防ぎます。`sub` はワークフロー実行 ID ではなく、Caller の不変な識別子を得るための根拠です。長さと文字種の条件は ID の一致による認可を強めるものではなく、OIDC ID Token の仕様と GATES の受理形式を明示するためのものです。

### 2.3 `aud`

**標準仕様**：RFC 7519 §4.1.3 では受信対象を文字列または文字列配列で表します。受信者自身が含まれなければ拒否することが MUST です。OpenID Connect Core §2 の ID Token は RP の `client_id` を含むことを要求しますが、GitHub Actions のトークン取得はその認証フローと異なります（[RFC 7519 §4.1.3](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.3)、[OIDC Core §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)）。

**GitHub 公式仕様**：既定の Audience はリポジトリ所有者の URL ですが、JWT 取得時にカスタム Audience を指定できます（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。

**実測値**：Audience を指定せずに取得した6件はリポジトリ所有者の URL、URL でない文字列を Audience に指定して取得した2件は指定した値の JSON 文字列でした（[GitHub Constraints](../spec/github-constraints.md#31-issueraudienceclaim)）。

**GATES の受理条件**：必須。固定設定の GATES 用 Audience と大文字小文字を含め完全一致する文字列1個、またはそれだけを含む要素数1の文字列配列を許容し、要素が複数なら GATES 用 Audience を含んでも拒否します。空文字列、空配列、別の Audience は拒否します。受信者を増やした JWT を資格情報として再利用しないための追加制約です。AWS 用 JWT の `aud=sts.amazonaws.com` とは区別します。

### 2.4 `exp`

**標準仕様**：RFC 7519 §4.1.4 は NumericDate を定め、受信時刻が `exp` より前でなければ受理できません。時計差の小さな猶予は MAY ですが必須ではありません。OIDC Core §2 は ID Token で `exp` を必須とします（[RFC 7519 §4.1.4](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.4)、[OIDC Core §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)）。

**実測値**：JSON 数値で、6件とも `iat` の300秒後です。RFC / GitHub 公式が一律に300秒の寿命を保証している意味ではありません。

**GATES の受理条件**：必須。JSON 数値の整数（小数点・指数表記も拒否）、範囲 `0..253402300799` のみ。時刻の判定は §2.38.3 を正本とし、有効期限の猶予は設けません。観測された5分寿命を GATES 固有の寿命上限にはしません。整数・範囲は GATES の追加制約です。

### 2.5 `iat`

**標準仕様**：RFC 7519 §4.1.6 は発行時刻を NumericDate とします。`iat` が未来であることだけを理由に拒否することは RFC 7519 の必須の受理規則ではありません。OIDC Core §2 は ID Token に `iat` を要求します（[RFC 7519 §4.1.6](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.6)）。

**実測値**：JSON 数値の整数です。

**GATES の受理条件**：必須。§2.4 と同じ JSON 整数・範囲を要求し、§2.38.3 の未来時刻の条件を適用します。発行時刻への5分の猶予は GATES 独自の判断です。

### 2.6 `iss`

**標準仕様**：RFC 7519 §4.1.1 の Issuer は JSON 文字列で、大文字小文字を区別します。OIDC Core §2 は ID Token で必須とし、クエリ・フラグメントのない HTTPS URL とします（[RFC 7519 §4.1.1](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.1)、[OIDC Core §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)）。

**GitHub 公式仕様・公開値**：GitHub Actions の Issuer は `https://token.actions.githubusercontent.com`。Discovery の `issuer` と一致します（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)、[Discovery 記録](../research/github/oidc-discovery.md)）。

**GATES の受理条件**：必須。固定文字列との完全一致。署名検証鍵は、この固定 Issuer の検証済み Discovery / JWKS だけから得ます。JWT 内の Issuer URL を取得先として使わず、Issuer の自由な切替えを許可しません。Discovery に異なる `issuer` があればその Metadata を利用しません（§5.3）。

### 2.7 `jti`

**標準仕様**：RFC 7519 §4.1.7 の任意 Claim。JWT の識別子であり、重複の可能性が無視できるように生成することが要求されます（[RFC 7519 §4.1.7](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.7)）。GitHub 公式資料は OIDC Token の一意な識別子と説明します。

**実測値**：6件すべてで文字列です。

**GATES の設計**：未使用。`jti` の存在、値、過去に使用したことの有無を認証・認可に用いません。JWT の有効期間内の再利用を検出しないのは [Requirements §2.2](../spec/requirements.md#22-out-of-scope) の明示的な対象外事項です。

### 2.8 `nbf`

**標準仕様**：RFC 7519 §4.1.5 は NumericDate とし、現在時刻が `nbf` 以降であること（等号は許容）を要求します。小さな時計差の猶予は MAY です。OIDC Core §2 の ID Token 必須 Claim にはありません（[RFC 7519 §4.1.5](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.5)）。

**GitHub 公式例と実測**：公式の JWT 例では `iat` の10分前ですが、6件の実測では5分前でした。時刻間隔の違いだけで標準違反とはなりません（[GitHub Constraints §3.1](../spec/github-constraints.md#31-issueraudienceclaim)）。

**GATES の受理条件**：任意・検証。欠落を許し、存在時は `exp` / `iat` と同じ JSON 整数・範囲および §2.38.3 の時刻条件を適用します。型違いを「欠落」として扱いません。

### 2.9 `ref`

**GitHub 公式仕様**：ワークフローを起動した Git ref（[GitHub OIDC reference — Custom claims](https://docs.github.com/en/actions/reference/security/oidc#custom-claims-provided-by-github)）。実測では `refs/heads/...` や `refs/pull/.../merge` を確認しています。

**GATES の設計**：任意・監査。存在し、JSON 文字列であればそのままワークフロー起動属性に保持します。欠落や文字列でない値は「記録なし」にし、認証・認可を拒否しません。ref 名をブランチポリシーやワークフローの信頼性の根拠として扱いません。

### 2.10 `sha`

**GitHub の根拠**：Discovery に掲載され、実測ではコミット SHA 形の文字列を確認しています。GitHub 公式の Token claims 表は `workflow_sha` を別に説明していますが、`sha` 自体は同表にありません。正確な意味を公式の同表の説明として引用しません（[GitHub Constraints §3.1](../spec/github-constraints.md#31-issueraudienceclaim)）。

**GATES の設計**：任意・監査。JSON 文字列ならワークフロー起動属性として記録し、欠落・型違いは拒否しません。40文字の16進数字であることを必須にせず、`workflow_sha` と同じであることも要求しません。

### 2.11 `repository`

**GitHub 公式仕様**：ワークフロー実行元のリポジトリ名です（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。実測は `owner/repo` 形式の文字列。

**GATES の設計**：未使用。`repository_id` の代用として Caller を確定しません。名前変更・移管・再利用があり得る名前に基づく認可をしないためです。存在・値を検証する独立した要件は設けません。

### 2.12 `repository_id`

**GitHub 公式仕様**：ワークフロー実行元のリポジトリの ID です。公式の JWT の例と実測6件では、JSON 数値でなく数字の文字列です。GitHub はすべての条件での型を明示していません（[GitHub Constraints §3.1](../spec/github-constraints.md#31-issueraudienceclaim)）。

**GATES の受理条件**：必須。§2.38.1 の形式を満たす正の整数の **JSON 文字列**。`sub` 内の Repository ID と一致すること。Caller の識別・監査に用いますが、現在のオーナー単位の発行認可には使いません（[Requirements FR-002](../spec/requirements.md#51-authentication)）。

### 2.13 `repository_owner`

**GitHub 公式仕様**：公式の Claim 表は、`repository` を格納する Organization 名と説明しています。しかし User オーナーへの同じ説明の適用は明記されていません。実測では User オーナー名と一致しました（[GitHub Constraints §3.1](../spec/github-constraints.md#31-issueraudienceclaim)）。

**GATES の設計**：未使用。認証・認可で使用する Owner ID を名前で代用しません。オーナーの名前変更で失敗する箇所（要求対象の完全名のオーナー部分と許可オーナー名の照合、オーナー名を含む IAM の `sub` 条件）は、この Claim を使わず、それぞれ別の設計の責務です（[Domain Model §4.4](domain.md#44-events-outside-gates-that-change-the-outcome)、§3.1）。

### 2.14 `repository_owner_id`

**GitHub 公式仕様**：公式の Claim 表では Organization ID と説明されています。User が所有するリポジトリでも実測6件でこの Claim が存在し、所有者の ID と一致しました。ただし、User 所有の場合の一般保証は公式資料から確定していません（[GitHub Constraints §3.1](../spec/github-constraints.md#31-issueraudienceclaim)）。

**GATES の受理条件**：必須。§2.38.1 の正の整数を示す JSON 文字列。`sub` 内の Owner ID と一致すること。Caller の Owner ID として認証し、固定した許可オーナーの Owner ID との照合に使用します（認可の条件は [Requirements FR-006](../spec/requirements.md#52-authorization)）。User 所有で取得できない場合は自動的に名前へフォールバックせず拒否します。

### 2.15 `enterprise`

**外部仕様・観測**：Discovery に掲載されていますが、参照した公式 Claim 表で意味・型・存在条件は確認できず、実測6件にも存在しません。

**GATES の設計**：未使用。存在・値を検証せず、将来利用するときに仕様を確認します。

### 2.16 `enterprise_id`

**外部仕様・観測**：Discovery に掲載。型・欠落条件・識別子の安定性は今回の根拠では確定できず、実測6件に存在しません。

**GATES の設計**：未使用。Claim 名が `_id` で終わるというだけで整数文字列と決めつけたり、固定オーナー ID に代用したりしません。

### 2.17 `run_id`

**GitHub 公式仕様**：ワークフロー実行の ID。`GITHUB_RUN_ID` 環境変数は再実行でも変わらないとされていますが、OIDC Claim と環境変数の一致がすべての条件で保証されるとは明記されていません（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)、[GitHub Variables reference](https://docs.github.com/en/actions/reference/workflows-and-actions/variables)）。実測は数字の JSON 文字列です。

**GATES の受理条件**：必須。§2.38.1 の正の整数文字列。監査上のワークフロー実行 ID に用い、発行認可には使用しません。`run_attempt` と組にして再実行の試行を区別します。

### 2.18 `run_number`

**GitHub 公式仕様**：ワークフローが実行された回数の番号を示します。実測は数字の JSON 文字列（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)、[実測記録](../research/github/oidc-jwt.md)）。

**GATES の設計**：未使用。欠落・値を検証しません。

### 2.19 `run_attempt`

**GitHub 公式仕様**：OIDC reference は「このワークフロー実行が再試行された回数」と説明します。一方 `GITHUB_RUN_ATTEMPT` 環境変数は初回を1として再実行で増加すると定められていますが、両者の等価性は明記されていません（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)、[Variables reference](https://docs.github.com/en/actions/reference/workflows-and-actions/variables)）。実測6件はすべて `"1"` で、再実行は実測していません。

**GATES の受理条件**：必須。§2.38.1 の正の整数文字列。`run_id` とともに監査に保持し、認可には使いません。`"0"` の拒否は、GitHub の Claim 一般に適用される保証や認可上の必要条件ではなく、GATES が対応する試行番号の形式を限定する判断です。初回に `"0"` を発行する条件があれば正当な JWT を拒否するため、初回・再実行の実測で互換性を確認します。

### 2.20 `actor`

**GitHub 公式仕様**：ワークフロー実行を開始した個人アカウント（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。

**GATES の設計**：任意・監査。JSON 文字列をワークフロー起動属性として保持します。欠落・型違いは拒否しません。Caller の所有者やリポジトリの所有者を `actor` の名前で代用しません。

### 2.21 `actor_id`

**GitHub 公式仕様**：ワークフロー実行を開始した個人アカウントの ID。実測は数字の JSON 文字列（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。

**GATES の設計**：任意・監査。JSON 文字列ならそのまま記録します。必須 ID Claim と異なり正の整数への変換を必須にせず、欠落・型違いを拒否しません。認可には使いません。

### 2.22 `workflow`

**GitHub 公式仕様**：ワークフローの名前です（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。

**GATES の設計**：未使用。名前で発行を制限しません。

### 2.23 `workflow_ref`

**GitHub 公式仕様**：ワークフローの参照パス（リポジトリ、ワークフローファイル、ref を含む）。実測では再利用ワークフローを実行しても、呼び出し元のワークフローを指しました（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)、[実測記録](../research/github/oidc-jwt.md)）。

**GATES の設計**：任意・監査。JSON 文字列を保存します。欠落・型違いは拒否せず、特定のワークフローファイルを要求する認可には使いません。`job_workflow_ref` と同一とは仮定しません。

### 2.24 `workflow_sha`

**GitHub 公式仕様**：ワークフローファイルのコミット SHA（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。再利用ワークフローの SHA とは区別されます。

**GATES の設計**：未使用。`sha`、`job_workflow_sha` との一致は検証しません。コミットの信頼性や認可の根拠に使いません。

### 2.25 `head_ref`

**GitHub 公式仕様**：Pull Request の元ブランチ（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。実測では Pull Request 以外でも空文字列で存在しました。

**GATES の設計**：未使用。空文字列を欠落と同じとみなす独自の必須条件は設けず、値で発行可否を制限しません。

### 2.26 `base_ref`

**GitHub 公式仕様**：Pull Request のターゲットブランチ（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。実測では Pull Request 以外でも空文字列で存在しました。

**GATES の設計**：未使用。`head_ref` と同様に、値の有無で認証・認可を変えません。

### 2.27 `event_name`

**GitHub 公式仕様**：ワークフローを起動したイベント名（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。

**GATES の設計**：任意・監査。JSON 文字列を保持し、`pull_request_target` 等を後から調査できるようにします。イベント種別で認可を拒否するポリシーは現時点で設けません。イベント名を見ただけでジョブ内のコードが信頼できるとは解釈しません。

### 2.28 `ref_type`

**GitHub 公式仕様**：`ref` の種別。例えば `branch`（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。

**GATES の設計**：未使用。`ref` との値の整合性を確認せず、認証・認可にも使用しません。

### 2.29 `ref_protected`

**外部仕様・実測**：Discovery に掲載されていますが、参照した GitHub OIDC reference の Claim 表には、型と意味の明示的な説明がありません。実測6件では JSON boolean ではなく文字列 `"false"` でした。

**GATES の設計**：未使用。Claim 名から保護ブランチ判定の正確な意味を推定して認可に使いません。将来使用する場合は、名前だけでなく GitHub 公式の保証と発行条件を確認します。

### 2.30 `environment`

**GitHub 公式仕様**：ジョブが使う Environment 名。GitHub 公式は、`environment` Claim を含む場合は Environment が必要であると説明します。実測6件のうち Environment を指定した1件だけに存在しました（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)、[実測記録](../research/github/oidc-jwt.md)）。

**GATES の設計**：任意・監査。存在し、JSON 文字列なら記録します。`environment` Claim のないジョブを拒否せず、Environment の名前から承認や保護規則の適用結果を推定しません。`sub` の `environment:` コンテキストを認可するためには用いません。

### 2.31 `environment_node_id`

**外部仕様・実測**：Discovery に掲載されています。実測では Environment を使う1件でのみ文字列が存在しました。公式資料から型・識別子の意味・出現条件の一般保証は確定していません。

**GATES の設計**：未使用。Environment の認可に使用せず、欠落を拒否しません。

### 2.32 `job_workflow_ref`

**GitHub 公式仕様**：再利用ワークフローを利用するジョブについて、その再利用ワークフローの参照パスを示します。実測では再利用しない場合にも存在し、`workflow_ref` と同じ値でした（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)、[実測記録](../research/github/oidc-jwt.md)）。

**GATES の設計**：任意・監査。JSON 文字列なら保持し、欠落・型違いは拒否しません。再利用する場合としない場合の存在条件を、6件の実測から保証された仕様へ格上げしません。`workflow_ref` との一致も要求しません。

### 2.33 `job_workflow_sha`

**GitHub 公式仕様**：再利用ワークフローファイルのコミット SHA。実測では再利用していないジョブにも存在し、再利用時には `workflow_sha` と異なる場合がありました（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)、[実測記録](../research/github/oidc-jwt.md)）。

**GATES の設計**：未使用。必須にもせず `sha` と照合しません。ワークフローの出自を認可に使う機能は現在のスコープ外です。

### 2.34 `repository_visibility`

**GitHub 公式仕様**：実行元リポジトリの可視性。列挙値は `internal`、`private`、`public`（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。

**GATES の設計**：未使用。private のみ許可する等の制約を導入せず、Claim の値や欠落を検証条件にしません。

### 2.35 `runner_environment`

**GitHub 公式仕様**：ランナー種別。`github-hosted` または `self-hosted`（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。

**GATES の設計**：未使用。GitHub-hosted かどうかでトークン発行を制限しません。ランナーが信頼できることの証明としては扱いません。

### 2.36 `issuer_scope`

**外部仕様・観測**：Discovery に名前が掲載されていますが、参照した GitHub 公式の Claim 表では意味、型、存在条件を確認できません。実測6件に存在しません。

**GATES の設計**：未使用。存在・値を検証しません。

### 2.37 `check_run_id`

**GitHub 公式仕様**：現在のジョブの Check Run ID（[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。実測では数字の JSON 文字列です。

**GATES の設計**：未使用。`run_id` や `run_attempt` の代用にせず、認証・認可・監査へ取り込みません。

### 2.38 Cross-Claim Invariants

各 Claim の値を独立に受理しただけでは Caller を確定しません。次の条件が同時に成立したときに限り、[Domain Model](domain.md#53-invariants) が定める Caller を生成できます。

#### 2.38.1 ID の表記

`repository_owner_id`、`repository_id`、`run_id`、`run_attempt` と `sub` のオーナー ID・リポジトリ ID はすべて、次の条件を満たす必要があります。

- JSON の **文字列** である。JSON 数値、`null`、boolean は拒否する。
- ASCII 数字 `[0-9]` だけで構成し、先頭に `0` を持たず、`"0"` を拒否する。`+` / `-`、空白、小数点、指数表記、非 ASCII 数字は拒否する。
- 64 bit 符号付き整数の正の範囲 `1..9223372036854775807` に収まる。
- この表記は GitHub 公式が保証した一般的な型・範囲ではなく、公式の JWT 例と実測値に基づいて GATES が固定した形式である。GitHub が型を変えたときに暗黙に受理を広げないため、形式変更は明示的な設計変更として扱う。

#### 2.38.2 Subject と識別子の一致

`sub` の構文・値の形式は §2.2、共通の ID 表記は §2.38.1 を正本とします。Caller を確定するには、さらに次の **Claim 間の一致** を要求します。

| 条件 | 不一致時の扱い |
|---|---|
| `sub` の Owner ID = `repository_owner_id` | 拒否（認証失敗） |
| `sub` の Repository ID = `repository_id` | 拒否（認証失敗） |

Caller の確定後、固定した Allowed Owner ID との一致は認可で確認します。

#### 2.38.3 時刻の判定

`now` を検証時の現在時刻（Unix 秒の整数）、`L=300` 秒とします。§2.4、§2.5、§2.8 の値を、同一の `now` で判定します。

| Claim | 存在 | GATES の判定 |
|---|---|---|
| `exp` | 必須 | `now < exp`。等号は拒否、猶予なし |
| `iat` | 必須 | `iat <= now + L` |
| `nbf` | 任意 | 存在するとき `nbf <= now + L` |

3つとも存在する場合でも、実測された `nbf=iat-300` や `exp=iat+300` の固定関係を要求しません。`exp > iat` のような追加の相互順序条件も、現行 GATES の受理規則としては設けません。これらを追加する場合は、外部仕様の要求か GATES の新しい制約かを明確にしたうえで別途判断します。

#### 2.38.4 JSON と Claim 名の一貫した解釈

- Claims Set は UTF-8 の JSON オブジェクトとし、不正な UTF-8 と、エスケープ解除後に同一となる Claim 名の重複を拒否する。未使用 Claim の重複も拒否する（RFC 7519 §4 が許容する処理）。
- Claim 名はエスケープ解除後の **厳密一致** で識別する。識別では、大文字小文字の同一視、Unicode 正規化、Unicode の Case Folding は行わない（[RFC 7519 §7.3](https://www.rfc-editor.org/rfc/rfc7519.html#section-7.3)、[OIDC Core §14](https://openid.net/specs/openid-connect-core-1_0.html#StringOps)）。`iss`、`ISS`、`iſſ` はそれぞれ別の名前として扱う。
- `iss`、`aud`、`exp`、`iat`、`nbf` の **別名** の Claim があれば、正しい `iss` などが同時にあっても拒否する。Requirements AC-27 に従う GATES の追加制約であり、JSON デコーダーが別名を同じ Claim として読む危険も防ぐ。`iss` と `\u0069ss` の同時存在は名前の重複として拒否する。
- 別名は、5 つの名前のどれかと Unicode の単純な Case Folding（[CaseFolding.txt](https://www.unicode.org/Public/UCD/latest/ucd/CaseFolding.txt) の状態 C と S）で同じ文字列に畳み込まれ、その名前自体とは異なる名前とする。5 つの名前の文字では、各 ASCII 英字の大文字と小文字、および `s`、`S`、`ſ`（U+017F）がそれぞれ同じ文字に畳み込まれる。別名は `iss` の 17 個（`ISS`、`iſſ`、`iſs` など）と、ほかの 4 つの名前の各 7 個（`AUD`、`eXp` など）の計 45 個である。
- 完全な Case Folding（状態 F）とトルコ語用の対応（状態 T）は使わない。そのため、`iß` と `İss` は別名ではなく、未知の Claim として扱う。`ıss` も、`ı`（U+0131）がどの Case Folding でも `i` に畳み込まれないため、未知の Claim として扱う。範囲を単純な Case Folding に揃えるのは、検証ライブラリの JSON デコーダーが Claim 名を同一視する範囲がこれと一致し、`iß` のような名前は `iss` として読まれないためである。
- 検証ライブラリと Claim 抽出処理で、同じ署名済み Claims Set の **同名・同値** を使用する。
- 任意・監査用 Claim は JSON 文字列として読み取れた場合だけ転送する。型違い・欠落は拒否せず、構造化ログではエスケープする。未知 Claim を自動的に監査へ取り込まない。

#### 2.38.5 Discovery に掲載されない動的 Claim

**GitHub 公式仕様**：Organization / Enterprise 管理者は Repository Custom Properties を OIDC JWT に追加できます。Claim 名は `repo_property_` とプロパティ名の連結です。文字列型と単一選択型は文字列、複数選択型は選択値のカンマ区切り文字列、真偽値型は JSON の boolean ではなく文字列の `"true"` / `"false"` として表現されます（[GitHub OIDC reference — Including repository custom properties in OIDC tokens](https://docs.github.com/en/actions/reference/security/oidc#including-repository-custom-properties-in-oidc-tokens)）。

**GATES の設計**：`repo_property_*` は現在すべて未使用です。存在・欠落・値・型を認証、認可、監査の根拠とせず、存在するだけで JWT を拒否しません。例えば `repo_property_business_unit` や `repo_property_pci_compliant` を追加しても、必須 Claim の検証結果は変えません。将来 GitHub が追加する未知 Claim も同様です。ただし Claims Set 全体の JSON の妥当性、名前の重複、署名検証は引き続き必要です。将来この属性を認可に使うときは、意味・型・信頼性・変更権限を別途評価し、受理条件を改訂します。

## 3. AWS 用 OIDC JWT

AWS 用 JWT の検証そのものは AWS STS が担当し、Lambda はこの JWT を受け取らず再検証しません。GATES は IAM Role の Trust Policy と許可する操作を設計します。具体的な JSON と Terraform 設定、`StringLike` の式、ワイルドカードの危険性、名前変更時の更新方法は AWS のリソース設計で定めます。

### 3.1 Claim Conditions

| Claim | 外部仕様 | GATES の設計 | 保証しないこと |
|---|---|---|---|
| `iss` | GitHub Actions OIDC の固定 Issuer | IAM OIDC Provider の信頼先を `https://token.actions.githubusercontent.com` に固定 | Lambda 内で AWS 用 JWT を検査すること |
| `aud` | GitHub で要求時に指定可能 | IAM で `sts.amazonaws.com` と一致させる | GATES 用 JWT の Audience まで保証すること |
| `sub` | GitHub の旧形式と不変形式が混在 | 許可オーナー名と不変 Owner ID を含む接頭部分を固定し、旧形式を拒否 | Repository ID とコンテキストの完全な構文検証 |
| その他 | GitHub が追加 Claim を提供し得る | 現在の Trust Policy の認可条件には使わない | その他の Claim が不正・欠落なら Lambda で検証されること |

## 4. GitHub App JWT

GitHub App JWT は、GitHub API に GitHub App として認証するために GATES が生成する JWS です。OpenID Connect ID Token ではないため、OIDC の `sub`、`aud`、`nonce` に関する Claim 要求を機械的に追加しません（[GitHub Constraints §4.2](../spec/github-constraints.md#42-github-app-jwt)、[GitHub App JWT 公式資料](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app)）。

### 4.1 Claim Summary

| Claim | GitHub 公式仕様 | GATES の生成条件 |
|---|---|---|
| `iss` | GitHub App の App ID または Client ID。Client ID 推奨 | 固定設定の GitHub App Client ID |
| `iat` | 発行時刻。60秒過去への設定を推奨 | 生成時刻から60秒前の NumericDate |
| `exp` | 失効時刻。現在から最大10分先 | 生成時刻の5分後の NumericDate |

### 4.2 `iss`

**GitHub 公式仕様**：GitHub App の App ID または Client ID を指定できます。GitHub は Client ID を推奨しています。

**GATES の生成条件**：デプロイ時の固定設定にある GitHub App Client ID を設定します。Installation ID、Owner ID、ワークフローの値を `iss` に使いません。GitHub API はこの発行者情報と署名鍵から App を検証します。

### 4.3 `iat`

**標準仕様**：RFC 7519 §4.1.6 の NumericDate。

**GitHub 公式仕様**：60秒過去に設定することは時計差対策の推奨で、MUST ではありません。

**GATES の生成条件**：生成時刻の60秒前を JSON 数値の整数として設定します。UTC の秒単位の値を用います。

### 4.4 `exp`

**標準仕様**：RFC 7519 §4.1.4 の NumericDate。

**GitHub 公式仕様**：現在時刻から最大10分先まで。

**GATES の生成条件**：生成時刻の5分後を JSON 数値の整数として設定します。GitHub に渡すときの有効期限は GitHub が検証するもので、GATES が利用者に返す Installation Access Token の有効期限とは別です。

5分は GitHub の上限の10分より短い値です。Lambda の時計が GitHub の時計より進んでいると、GitHub からは `exp` が5分より先に見えて10分の上限に近づき、遅れていると期限が早く来ます。5分はどちらの向きにも数分のずれを許します。生成から使い終えるまでの時間がこれより短いことは、GitHub App JWT を使うコンポーネントの設計が確かめます。

### 4.5 Header と署名

JOSE Header は `alg` と `typ` の2つだけとし、`alg` は `RS256`、`typ` は `JWT` に固定します。Base64URL（paddingなし）の JWS Compact Serialization として生成します。`RS256` は SHA-256 と RSA PKCS#1 v1.5 署名の組です。

`typ` は RFC 7519 §5.1 で任意ですが、置きます。GitHub が Installation の取得を受け付けた調査記録の GitHub App JWT がこの Header だったためです（[GitHub App Installation](../research/github/app-installation.md)）。`typ` のない GitHub App JWT を GitHub が受け付けるかは確かめていません。

## 5. Common Serialization, Header, Keys and Discovery

### 5.1 JWS と JSON の基本条件

**標準仕様**：JWT は JWS / JWE の形式で表せますが、GATES が検証する GitHub OIDC JWT は署名付き JWS Compact Serialization に限ります。JWS は Protected Header、Payload、Signature の3部分を `.` で連結し、Base64URL で表します（[RFC 7515 §3.1、§5.2](https://www.rfc-editor.org/rfc/rfc7515.html#section-5.2)、[RFC 7519 §1](https://www.rfc-editor.org/rfc/rfc7519.html#section-1)）。Header と Claims Set に UTF-8 を使うことは RFC 8725 §3.7 の MUST です。

**GATES の受理条件**：JWS Compact Serialization だけを受理し、3部分以外の形、JSON Serialization、Base64URL 以外の文字、余分な padding・空白は拒否します。Header / Claims Set で、エスケープ解除後に同一となる JSON 名の重複や不正な UTF-8 も拒否します。未対応の JWE は受理しません。

JWT の生の文字列に CR / LF を含む場合も拒否します。JWS Compact Serialization の符号化された構成要素に改行を含めることはできず、検証ライブラリが読み飛ばす挙動を受理条件の代用にしません。

### 5.2 JOSE Header

| Header | 標準仕様 | GATES の受理条件・扱い |
|---|---|---|
| `alg` | JWS で必須。実際の署名方式と一致 | 必須。`RS256` のみ。`none`、`PS256` 等は拒否 |
| `kid` | JWS 一般では任意。OIDC Core §10.1 では JWKS に複数鍵があるとき必須 | 必須。JWKS 内の使用可能な鍵を選択するために利用。値は大文字小文字を区別 |
| `typ` | 任意。JWT の種類識別に使える | 存在を必須にしない。署名鍵の取得先を決めない |
| `cty` | RFC 7519 §5.2 で Nested JWT を示す場合は `JWT`（大小文字を区別しない）が必要 | GATES は Nested JWT に対応せず、`cty` が存在する JWT は値を問わず拒否する |
| `crit` | 未対応の拡張があれば無効 | GATES は拡張を実装しないため、存在すれば拒否 |
| `b64` | RFC 7797 は `b64:false` を JWT で禁止 | `true` / `false` とも GATES が扱う JWT プロファイルでは受理しない |
| `jku`, `jwk`, `x5u`, `x5c`, `x5t`, `x5t#S256` | 署名鍵情報に関係する任意の Header | 存在を拒否理由にせず、鍵の取得先・検証鍵選択の根拠にしない |
| その他の未理解 Header | 一般には無視することが MUST | 無視する。未知であることだけでは拒否しない |

`cty` を一律拒否するのは、Nested JWT に対応しない GATES の JWS プロファイルの追加制約です。`kid` は外部 URL やファイルパスとして解釈せず、固定した Issuer の JWKS 内で鍵を引くためだけに使います。`kid` を特定の UUID / 16進数字形式に限定しません（実測の JWKS でも形式は混在）。`crit` が空・不正であっても受理せず、未対応の拡張が意味する処理を無視しません。`b64:true` の拒否は標準による一律禁止ではなく GATES の追加制約です（[RFC 7515 §4](https://www.rfc-editor.org/rfc/rfc7515.html#section-4)、[RFC 7797 §6–7](https://www.rfc-editor.org/rfc/rfc7797.html#section-7)）。

### 5.3 Discovery と鍵の取得先

GitHub の Discovery 公開値は `issuer`、`jwks_uri`、`subject_types_supported`、`response_types_supported`、`claims_supported`、`id_token_signing_alg_values_supported`、`scopes_supported` で、`authorization_endpoint` と `token_endpoint` を持ちません（[調査記録](../research/github/oidc-discovery.md)）。

**標準仕様との境界**：OpenID Connect Discovery §3 は `authorization_endpoint` を REQUIRED とし、`token_endpoint` は Implicit Flow だけを使用する場合を除いて REQUIRED とします。この GitHub 公開値は完全な OpenID Provider Metadata の要求を満たしません。ただし、Metadata の不足だけで個々の JWT の署名・Claim が不正だと断定することはできません（[OpenID Connect Discovery §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)、[JWT Constraints §5](../spec/jwt-constraints.md#5-openid-connect-and-discovery)）。

**GATES の設計**：固定 Issuer の Discovery を HTTPS で取得し、**一致を確認した `issuer` と JWKS の取得先 `jwks_uri`** だけを使用します。認証 Endpoint、`client_id`、`nonce` を前提にした通常の OpenID Connect 認証フローは実行しません。JWKS の URL は Issuer と同じ HTTPS のホスト・ポートに制限し、別ドメインへリダイレクトしません。JWT の Header やリクエストが任意の鍵取得 URL を指定することも許しません。

取得時刻、キャッシュの存続、JWKS の再取得上限、タイムアウト、応答サイズ、失敗時の戻り値は本書の受理条件ではなく、検証器のコンポーネント設計が扱います。完全な Discovery クライアントとして標準準拠しているとは主張しません。

### 5.4 JWK と JWK Set の受理条件

**標準仕様**：JWK Set は `keys` 配列を持つ JSON オブジェクトです。RSA 公開鍵では `kty=RSA`、`n`、`e` を用います。`use` と `key_ops` の併用・整合性、`RS256` の RSA 鍵長については [RFC 7517 §4–5](https://www.rfc-editor.org/rfc/rfc7517.html#section-4)、[RFC 7518 §3.3・§6.3](https://www.rfc-editor.org/rfc/rfc7518.html#section-6.3) に従います。OpenID Connect Discovery §3 は Provider に対し、JWKS に秘密鍵・対称鍵の値を含めることを **MUST NOT** とします（[Discovery §3](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata)）。GATES では、この違反を含む JWKS 全体を拒否する追加制約を採用します。

**GATES の鍵採用条件**：次の条件をすべて満たし、RSA 公開鍵として検証処理に渡せる JWK を **使用可能な鍵** とします。これは署名の一致を意味しません。個別条件に適合しない JWK は **除外した JWK** とし、保持しません。

- `kty=RSA`。`kid` は空でない JSON 文字列であり、`n` と `e` は正しい Base64urlUInt として復号できる。
- RSA の法 `n` は2048 bit 以上の正の奇数、公開指数 `e` は3以上の奇数。`e` は `2^31 − 1` 以下とする。奇偶と下限は RSA 公開鍵としての妥当性を確保し、上限は署名の検証で拒否される鍵を保持して署名の不一致に見えないようにする GATES の追加制約であり、RFC 7518 の `n` / `e` の Base64urlUInt という表現要件とは区別する。
- `use` が存在すれば `sig`、`key_ops` が存在すれば文字列の配列で `verify` を含み、重複がない。`use` と `key_ops` が両方存在する場合は、`use=sig` と整合させるため、`key_ops` の値を `sign` と `verify` に限る（`["verify"]` と `["sign", "verify"]` を受理する）。`alg` が存在すれば `RS256`。
- `x5c` が存在する場合は §5.4.1 の証明書との整合条件も満たす。`x5c` 自体は必須ではない。

法の素因数分解など、公開鍵だけでは確認できない性質まで保証するものではありません。採用した公開鍵は、最後に `RS256` の実際の署名検証に使用します。公開鍵の構築に失敗する場合は署名不一致と読み替えず、除外した JWK として扱います。

#### 5.4.1 `x5c` と証明書の整合

**標準仕様**：RFC 7517 §4.7 は、`x5c` を Base64（Base64URL ではない）で表した X.509 証明書の配列とし、先頭証明書の公開鍵が JWK の公開鍵パラメーターと一致することを **MUST** とします。`use`、`alg` などの関連メンバーも、証明書に記載された対応情報と意味的に整合することが **MUST** です（[RFC 7517 §4.6–§4.7](https://www.rfc-editor.org/rfc/rfc7517.html#section-4.7)）。

**GATES の受理条件**：`x5c` があれば、1件以上の文字列を含む配列として読み、各要素が Base64 DER の X.509 証明書として解析できることを要求します。先頭証明書には RSA 公開鍵を要求し、その `n` / `e` を JWK と一致させます。2件目以降がある場合は、後続の証明書が直前の証明書の発行者であることを署名関係で確認します。証明書に Key Usage がある場合は、署名用途と矛盾しない `digitalSignature` を要求します。`alg=RS256` は証明書の公開鍵アルゴリズム・用途と整合させ、**証明書自身の発行者による署名アルゴリズムが `RS256` と同じであることは要求しません**。これらに不一致があれば鍵を除外します。RFC 7517 §4.6 は `alg` が「証明書に指定されたアルゴリズム」と対応することを求めますが、どのフィールドかを定めていません。GATES はこれを公開鍵のアルゴリズムと解釈します。発行者の署名アルゴリズムは発行者が証明書に署名した方式であり、この鍵で JWT を検証する方式ではないためです。

`x5c` の証明書を新たな信頼アンカーにはせず、PKIX の認証パス全体の信頼性やルート証明書の信頼性を判定する方式も採りません。署名検証鍵は引き続き固定 Issuer の JWKS によって信頼します。

#### 5.4.2 秘密鍵・対称鍵とエラー分類

**秘密情報を含む集合**：JWKS 内のいずれかの JWK が `kty=oct`、または `kty` を問わず秘密鍵・対称鍵の値のメンバー（対称鍵の `k`、秘密鍵の `d`、`p`、`q`、`dp`、`dq`、`qi`、`oth`）を含む場合は、名前の重複などでその JWK を JWK として読めなくても、またほかに正常な鍵があっても、JWKS 全体を拒否し、内部障害とします。該当する値はログ・エラー応答に出しません。未知の公開情報メンバーは、存在するだけでは拒否しません。

**集合・鍵・JWT の失敗分類**：取得・キャッシュ・再取得の手順はコンポーネント設計が担当します。最終的に検証対象となる JWKS の分類は次のとおりです。

| 条件 | GATES の扱い・最終エラー |
|---|---|
| JWKS の JSON 不正、`keys` が配列でない、集合ルートに名前の重複がある | 集合全体を拒否。内部障害 |
| 秘密鍵・対称鍵の値が含まれる | 集合全体を拒否。内部障害 |
| 個別 JWK の構文・型・公開鍵構築・`x5c` 整合の失敗 | その鍵を除外。残りの使用可能な鍵で判定を続ける |
| 除外後に使用可能な鍵が0件 | 検証基盤の異常。内部障害 |
| JWT の `kid` に一致する使用可能な鍵がなく（`kid` が元の JWKS に存在しない、または除外した JWK にだけ一致）、ほかに使用可能な鍵がある | 再取得規則を経て認証失敗 |
| JWT の `kid` が使用可能な鍵に複数一致 | 一意に選べないため、再取得規則を経て認証失敗。総当たり検証しない |
| JWT の `kid` が使用可能な鍵に1件一致したが署名が不一致 | 再取得規則を経て認証失敗 |
| Discovery / JWKS の通信失敗・HTTP エラー・読み取り失敗 | JWT 自体の不正とは区別し、コンポーネント設計の外部障害分類に従う |

同じ `kid` の除外した JWK と使用可能な鍵が混在しても、使用可能な鍵が1件だけならその鍵を選びます。除外した JWK は、JWT の判定とは別に、運用者が Issuer の鍵情報の不整合に気付けるよう記録します。記録の方法はコンポーネント設計で定めます。

**観測と限界**：取得した JWKS は4件とも2048 bit RSA 公開鍵で、`x5c` / `x5t` は一部にのみ存在し、`kid` の形式も混在しています（[GitHub OIDC JWKS](../research/github/oidc-jwks.md)）。3件の `x5c` の証明書公開鍵が `n` / `e` と一致することは調査記録を用いて検証済みです。この観測を将来の JWKS の保証とはみなしません。

## 6. Traceability and Verification Cases

### 6.1 要求との対応

| GATES の検証・生成契約 | 要求・設計上の根拠 |
|---|---|
| 必須 Claim、Issuer、Audience、時間、署名と ID の整合 | [Requirements FR-002、FR-003](../spec/requirements.md#51-authentication)、[Domain Model §5.3](domain.md#53-invariants) |
| 不変形式 `sub` と AWS STS の入口制御 | [Requirements FR-005、FR-025](../spec/requirements.md#51-authentication) |
| Caller の Owner ID による認可と Repository ID の監査利用 | [Requirements FR-002](../spec/requirements.md#51-authentication)、[FR-006](../spec/requirements.md#52-authorization) |
| 起動属性のベストエフォート監査 | [Requirements FR-021](../spec/requirements.md#54-usability-and-operations) |
| GitHub App JWT の生成、秘密鍵の非抽出、KMS 署名 | [Requirements FR-013](../spec/requirements.md#53-token-issuance) |
| 署名鍵の固定 Issuer からの取得 | [JWT Constraints §6](../spec/jwt-constraints.md#6-jwt-security-considerations)、[Requirements FR-002](../spec/requirements.md#51-authentication) |

### 6.2 実装・レビュー時の代表テスト

| 対象 | 受理例・拒否例の焦点 | 期待結果 |
|---|---|---|
| `sub` | 不変形式、旧形式、`@` の過不足、`:` 欠落、カスタマイズ形 | 構文と ID 一致を満たす場合だけ受理 |
| `sub` の長さ・文字種 | ASCII 255文字、256文字、非 ASCII、長い ref / Environment | ASCII 255文字以内のみ受理。長い値の GitHub 互換性は別途確認 |
| `aud` | GATES 用1件、文字列1件と配列1件、配列に空要素や別 Audience を含む場合、AWS 用 Audience | GATES 用の1要素だけを受理 |
| NumericDate | 数値整数、数字文字列、小数、指数、範囲外、`exp=now`、`exp=now+1` | 整数・範囲・各時刻条件を満たすものだけ受理 |
| 不変 ID | `"1"`、`"0"`、`"01"`、JSON 数値、符号、64 bit 範囲外、`sub` との不一致 | 正の整数文字列および一致だけ受理 |
| `run_attempt` | `"1"`、`"0"`、欠落、再実行時の値 | `"0"` は拒否。再実行時の実測は別途実施 |
| 任意・監査 | `environment` 欠落、監査 Claim の型違い、制御文字入りの文字列 | 認証は継続し、ログでは安全に記録 |
| Claim 名の比較 | `iss`、`ISS`、`iſſ`、`iſS` のような混在、`iß`、`İss`、`ıss`、`\u0069ss`、同時存在 | 厳密一致。§2.38.4 の 45 個の別名は拒否し、`iß`、`İss`、`ıss` は別名として扱わない。エスケープ解除後の重複は拒否 |
| 動的 Claim | `repo_property_*` の文字列・複数選択・真偽値文字列、未知 Claim | 追加 Claim があっても、必須 Claim が成立すれば認証を妨げない |
| JSON の曖昧さ | 重複 Claim 名、Unicode エスケープによる重複、不正 UTF-8 | 署名が正しくても拒否 |
| JOSE Header | 未知 Header、`crit`、`b64`、`cty=JWT` / `cty=jwt` / その他の `cty`、`alg=none`、`kid` 欠落、改行入り JWT | 未知 Header は無視し、`cty` を含む未対応の形式・拡張を拒否 |
| JWKS | `kid` 不在、除外した JWK だけが一致、複数一致、公開指数の境界、`x5c`・Key Usage の不整合、鍵ゼロ、秘密情報混入、取得障害 | §5.4 のエラー分類に一致し、すべて Fail Closed |
| IAM の境界 | 不変 Owner ID 入りの `sub`、旧形式、別オーナーの `sub` | 許可オーナーの接頭部分を制限し、全構文の検証とは区別 |
| GitHub App JWT | Client ID、`iat=now-60`、`exp=now+300`、`alg=RS256` と `typ=JWT` だけの Header、RS256/KMS | GitHub 公式上限を満たす JWT を生成 |

ケースの詳細なテスト実装とモック方式はコンポーネント設計が担当します。この表は受理条件のテスト可能性の索引であり、コードのテスト一覧の正本を二重に設けるものではありません。

## 7. Outstanding Verification

採用済みの受理・生成条件は §2〜§5 を正本とします。外部仕様・互換性の未確認事項は次のとおりです。

| ID | 未確認事項 | 確認方法 |
|---|---|---|
| JV-01 | 長い ref / Environment や Unicode により、GitHub が255 ASCII文字を超える `sub` を発行する条件 | 255/256文字境界を含め GitHub Actions で実測 |
| JV-02 | `run_attempt` の初回・再実行時の Claim 値と `GITHUB_RUN_ATTEMPT` との一致 | 初回と再実行を記録し比較 |
| JV-03 | User 所有リポジトリの `repository_owner_id` の一般保証 | GitHub 公式資料の確認と条件を変えた実測 |
| JV-04 | `repo` だけを指定する `sub` カスタマイズの末尾形式 | 不変形式の設定を有効にしたジョブで実測 |
| JV-05 | `sha`、`ref_protected`、`enterprise` 等の型・意味の一般保証 | 認可用途を追加する際に公式仕様と条件付き実測を確認 |
| JV-06 | 複数証明書を含む `x5c` の実測がない | 証明書間の署名関係を含むテストで受理条件を確認 |

## 8. Sources

- [JWT Constraints](../spec/jwt-constraints.md) — 規範と適用範囲の整理
- [GitHub Constraints](../spec/github-constraints.md) — GitHub 公式資料、公開値・実測、相違・未確認事項
- [GitHub OIDC Discovery](../research/github/oidc-discovery.md) — `claims_supported`
- [GitHub OIDC JWKS](../research/github/oidc-jwks.md) — 鍵情報
- [GitHub OIDC JWT](../research/github/oidc-jwt.md) — 6件の JWT 観測
- [GitHub OIDC Custom Audience](../research/github/custom-audience.md) — Audience を指定した JWT の観測と、GitHub が発行を拒否した Audience
- [RFC 7515 — JSON Web Signature](https://www.rfc-editor.org/rfc/rfc7515.html)
- [RFC 7517 — JSON Web Key](https://www.rfc-editor.org/rfc/rfc7517.html)
- [RFC 7518 — JSON Web Algorithms](https://www.rfc-editor.org/rfc/rfc7518.html)
- [RFC 7519 — JSON Web Token](https://www.rfc-editor.org/rfc/rfc7519.html)
- [RFC 7797 — JWS Unencoded Payload Option](https://www.rfc-editor.org/rfc/rfc7797.html)
- [RFC 8725 — JSON Web Token Best Current Practices](https://www.rfc-editor.org/rfc/rfc8725.html)
- [OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)
- [OpenID Connect Discovery 1.0](https://openid.net/specs/openid-connect-discovery-1_0.html)
- [Unicode Character Database — CaseFolding.txt](https://www.unicode.org/Public/UCD/latest/ucd/CaseFolding.txt)（Claim 名の別名の範囲）
- [GitHub — OpenID Connect reference](https://docs.github.com/en/actions/reference/security/oidc)（動的 `repo_property_*` の公式仕様を含む）
- [GitHub — Generating a JWT for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app)
