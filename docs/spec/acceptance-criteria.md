# Acceptance Criteria

## Purpose

本ドキュメントは、[Requirements](requirements.md) に定めた要求に対する受け入れ条件を定め、各 AC を受け入れ条件の正本とします。
各 AC の Relationship の表は、その AC が適合を判定する要求を `alignment` で示します。
Target と本文に書く `FR-` と `QR-` の ID は、Requirements の要求を指します。

リポジトリポリシーを適用するデプロイで判定するときは、リポジトリポリシーに固有の AC（AC-AUTHZ-014 から AC-AUTHZ-017、AC-ISSUANCE-011、AC-ISSUANCE-012）と、AC-QUALITY-001 のうちリポジトリポリシーの取得の一時的な障害の場合を除き、各 AC の条件に、要求の対象のリポジトリポリシーがその要求を許していることを含めます。
リポジトリポリシーが要求を許していれば、各 AC の期待する結果は変わらないためです（FR-AUTHZ-011）。
リポジトリポリシーが許さない要求と、リポジトリポリシーの取得の一時的な障害が起きる要求の結果は、リポジトリポリシーに固有の AC と AC-QUALITY-001 で判定します。

## Authentication

### AC-AUTHN-001 未認証か許可されない呼び出し元の要求の拒否

**条件**：未認証、または許可されない呼び出し元からの要求

**期待する結果**：発行処理に入る前に拒否

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHN-002` | Functional Requirement |
| `alignment` | `FR-AUTHZ-001` | Functional Requirement |

### AC-AUTHN-002 JWT の欠落と、改ざん、期限切れ、Audience 不一致、必須 Claim の欠落・不正型の JWT の拒否

**条件**：JWT の欠落、改ざん、期限切れ、Audience 不一致、必須 Claim の欠落・不正型

**期待する結果**：発行を拒否

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHN-001` | Functional Requirement |
| `alignment` | `FR-AUTHN-002` | Functional Requirement |

### AC-AUTHN-003 `sub` の先頭で許可オーナーの不変 Owner ID を示さない JWT による IAM Role の引き受けの拒否

**条件**：`sub` が呼び出し元を名前だけで特定する形式の OIDC JWT、`sub` のカスタマイズにより先頭で不変 Owner ID を示さなくなった OIDC JWT、または `sub` の先頭が別のオーナーの不変 Owner ID を示す OIDC JWT を持つワークフローが、IAM Role の引き受けを試みる

**期待する結果**：引き受けが拒否され、Lambda は呼び出されない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHN-003` | Functional Requirement |

### AC-AUTHN-004 検証に使う Claim を一つの解釈で読み取れない JWT の拒否

**条件**：適切に署名された JWT に、GATES の検証で Claim を読み取る処理によっては、検証に使う Claim と同じ名前に読める別の名前の Claim を含める、または時刻の Claim の値を、GATES の検証で Claim を読み取る処理によって異なる値に読める表記で書く

**期待する結果**：ほかの条件を満たしていても、認証失敗として拒否し、Caller を確定しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHN-001` | Functional Requirement |
| `alignment` | `FR-AUTHN-002` | Functional Requirement |
| `alignment` | `FR-AUTHN-004` | Functional Requirement |

### AC-AUTHN-005 未対応の JOSE Header の拡張か検証に適さない鍵を用いた JWT の拒否と、検証鍵を得られない障害の区別

**条件**：GATES が対応しない JOSE Header の拡張を使う JWT を送る。Issuer が公開する検証鍵の集合に、使用できる鍵とともに、署名の検証の用途か署名方式に合わない鍵か、署名方式が求める強度を満たさない鍵を置き、その鍵に対応する秘密鍵で署名した JWT を送る。また、検証鍵の集合を取得できない障害と、その集合に使用できる鍵が一つもない障害を起こす

**期待する結果**：GATES が対応しない JOSE Header の拡張か、検証に適さない鍵を用いた JWT は、認証失敗として拒否し、検証に適さない鍵は検証鍵として採用しない。検証鍵の集合を取得できない障害と、使用できる鍵が一つもない障害は、認証失敗とは別のエラーとする

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHN-001` | Functional Requirement |
| `alignment` | `FR-AUTHN-002` | Functional Requirement |
| `alignment` | `FR-OPS-004` | Functional Requirement |
| `alignment` | `FR-AUTHN-005` | Functional Requirement |

## Authorization

### AC-AUTHZ-001 オーナー名が一致して Owner ID が異なる JWT の拒否

**条件**：JWT の不変 Owner ID が不一致（オーナー名が一致していても）

**期待する結果**：発行を拒否

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-001` | Functional Requirement |

### AC-AUTHZ-002 Payload での Caller、Installation ID、認可設定、API Host の指定の拒否

**条件**：Caller・Installation ID・任意の認可設定・API Host を Payload で指定

**期待する結果**：入力を受け付けず拒否

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-002` | Functional Requirement |

### AC-AUTHZ-003 Installation の対象外のリポジトリか、他のオーナーの Private Repository の要求の拒否

**条件**：Installation の対象外、または他のオーナーの Private Repository を要求

**期待する結果**：発行を拒否

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-005` | Functional Requirement |

### AC-AUTHZ-004 許可権限にない権限と全リポジトリの要求の拒否

**条件**：Organization permissions、Account permissions、Enterprise permissions のいずれかの権限、GATES が許可する権限名にない権限名、または許可する権限名に GATES がその権限名に許可しない水準を組み合わせた権限を、単独で、あるいは許可される権限とともに要求する。または全リポジトリ（`all`）を対象にする `repository_selection` を要求に含める

**期待する結果**：発行を拒否し、許可される権限だけに縮小して発行しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-006` | Functional Requirement |
| `alignment` | `FR-AUTHZ-003` | Functional Requirement |

### AC-AUTHZ-005 リポジトリか権限が欠落、空、未知値の要求の拒否

**条件**：リポジトリや権限が欠落、空、未知値

**期待する結果**：発行を拒否し、広い既定値を適用しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-003` | Functional Requirement |

### AC-AUTHZ-006 一部が発行できない複数対象の要求の全件拒否

**条件**：複数対象の一部が Installation の対象外、別のオーナー、または別 Installation

**期待する結果**：全件拒否し、暗黙に分割・縮小しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-005` | Functional Requirement |
| `alignment` | `FR-AUTHZ-004` | Functional Requirement |

### AC-AUTHZ-007 実行時の入力による許可オーナー、GitHub App、Installation、KMS Key、発行上限、保護対象の一覧、リポジトリポリシーの適用と取得先、ポリシー読み取りトークンの権限と対象の指定の拒否

**条件**：実行時の要求やワークフロー側の入力で、許可オーナー・GitHub App・Installation・KMS Key・発行上限・保護対象の一覧・リポジトリポリシーの適用・リポジトリポリシーのパス・リポジトリポリシーの ref・リポジトリポリシーの取得先・ポリシー読み取りトークンの権限・ポリシー読み取りトークンの対象の追加を指定しようとする

**期待する結果**：指定を受け付けず、デプロイ時の設定と、そこから導出した Installation が使われる。リポジトリポリシーを適用するデプロイでは、リポジトリポリシーは、要求の対象の既定ブランチの、GATES が定める固定のパスから読み、ポリシー読み取りトークンは、権限を `single_file:read` だけ、対象を要求の対象だけに限って取得する

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-002` | Functional Requirement |
| `alignment` | `FR-OPS-001` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-007` | Functional Requirement |

### AC-AUTHZ-008 GitHub App に付与されていない権限を含む要求の全件拒否

**条件**：GATES が許可する権限のうち、GitHub App の Installation に付与されていない権限か、付与された水準より高い水準を、単独で、あるいは付与されている権限とともに要求する

**期待する結果**：トークンを返さず、権限を除いたり水準を下げたりしたトークンを発行しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-007` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-004` | Functional Requirement |

### AC-AUTHZ-009 発行上限の水準による要求の発行と全件拒否

**条件**：GATES が許可する権限のうち GitHub App の Installation に付与された権限を、次のように要求する。発行上限に含まれる権限だけを、発行上限と同じ水準で要求する。発行上限に高い水準だけを含めた権限名の、低い水準を要求する。発行上限に低い水準だけを含めた権限名の高い水準か、発行上限にない権限名を、単独で、あるいは発行上限に含まれる権限とともに要求する

**期待する結果**：発行上限に高い水準だけを含めた権限名の低い水準の要求と、発行上限に含まれる権限だけを同じ水準で求める要求は、要求した水準で発行し、発行上限の水準に上げない。発行上限に低い水準だけを含めた権限名の高い水準か、発行上限にない権限名を含む要求は、発行を拒否し、発行上限に含まれる権限だけに縮小して発行しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-008` | Functional Requirement |
| `alignment` | `FR-AUTHZ-004` | Functional Requirement |

### AC-AUTHZ-010 GATES が許可しない権限か空の集合を発行上限にした設定の拒否

**条件**：GATES が許可しない権限名と水準の組（Organization permissions、許可する権限名にない権限名、許可する権限名に許可しない水準）を含む発行上限、空の発行上限、または書式の誤った発行上限でデプロイし、発行を要求する

**期待する結果**：どの要求にもトークンを返さない。設定の誤りは、値を記録せずに、どの設定の誤りかを運用者が特定できる形で残る

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-008` | Functional Requirement |
| `alignment` | `FR-OPS-001` | Functional Requirement |
| `alignment` | `QR-003` | Quality Requirement |

### AC-AUTHZ-011 保護対象への要求の許可と拒否

**条件**：空でない保護対象の一覧でデプロイし、GATES が許可し、発行上限に含まれ、GitHub App の Installation に付与された権限で、次の要求をする。許可 Caller に当たる Caller が、その許可 Caller の権限の上限と同じかより低い水準だけで保護対象を要求する。許可 Caller に当たらない Caller（Repository ID が一致してリポジトリ名が一致しない Caller と、リポジトリ名が一致して Repository ID が一致しない Caller を含む）が保護対象を要求する。許可 Caller に当たる Caller が、権限の上限より高い水準か、権限の上限にない権限名を含めて保護対象を要求する。許可 Caller のない保護対象を要求する。これらの保護対象への要求は、保護対象でない対象を混ぜない形と混ぜた形の両方で行う。保護対象を含まない要求をする

**期待する結果**：許可 Caller に当たる Caller の、権限の上限と同じかより低い水準だけの要求は、保護対象でない対象を混ぜていても、要求した水準で発行する。それ以外の保護対象を含む要求は、保護対象でない対象を混ぜていても全体を認可拒否とし、発行 API を呼ばない。保護対象を含まない要求の結果は、空の保護対象の一覧でデプロイした場合と同じである

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-009` | Functional Requirement |
| `alignment` | `FR-AUTHZ-004` | Functional Requirement |
| `alignment` | `FR-AUTHN-001` | Functional Requirement |

### AC-AUTHZ-012 リポジトリ名と Repository ID の対応が食い違う保護対象を含む要求へのトークンの不返却

**条件**：保護対象を改名した後に、保護対象の一覧を直さずに、許可 Caller に当たる Caller からも当たらない Caller からも、新しい名前で要求する。保護対象を削除して同じ名前で作り直した後に、保護対象の一覧を直さずに、許可 Caller に当たる Caller からその名前で要求する

**期待する結果**：トークンを返さない。空でないトークン文字列を取得した場合は失効 API を試み、成否を監査する。失効成功は保証しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-010` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-004` | Functional Requirement |

### AC-AUTHZ-013 不正な保護対象の一覧の設定の拒否

**条件**：保護対象の一覧が欠落した設定、書式の誤った一覧、保護対象の Repository ID かリポジトリ名が重複した一覧、同じ保護対象の中で許可 Caller の Repository ID かリポジトリ名が重複した一覧、空の権限の上限を持つ許可 Caller を含む一覧、GATES が許可しない権限名と水準の組を含む権限の上限を持つ許可 Caller を含む一覧のいずれかでデプロイし、発行を要求する

**期待する結果**：どの要求にもトークンを返さない。設定の誤りは、値を記録せずに、どの設定の誤りかを運用者が特定できる形で残る

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-OPS-001` | Functional Requirement |
| `alignment` | `FR-AUTHZ-009` | Functional Requirement |
| `alignment` | `QR-003` | Quality Requirement |

### AC-AUTHZ-014 リポジトリポリシーによる要求の許可と拒否

**条件**：リポジトリポリシーを適用するデプロイで、GATES が許可し、発行上限に含まれ、GitHub App の Installation に付与された権限で、保護対象でない対象に次の要求をする。対象のリポジトリポリシーの許可 Caller に当たる Caller が、その許可 Caller の権限の上限と同じかより低い水準だけで要求する。Caller が、リポジトリポリシーが Caller を許していない自分のリポジトリを要求する。リポジトリポリシーのない対象を要求する。許可 Caller に当たらない Caller（Repository ID が一致してリポジトリ名が一致しない Caller と、リポジトリ名が一致して Repository ID が一致しない Caller を含む）が要求する。許可 Caller に当たる Caller が、権限の上限より高い水準か、権限の上限にない権限名を含めて要求する。許可 Caller のないリポジトリポリシーの対象を要求する。拒否されるはずのこれらの要求は、リポジトリポリシーが要求を許す対象を混ぜない形と混ぜた形の両方で行う。また、許可 Caller に当たる Caller の要求が発行された後に、リポジトリポリシーをその Caller を許さない内容に変えて、同じ要求をする。固定のパスに、Caller を許す内容のファイルを指すシンボリックリンクを置いて要求する

**期待する結果**：許可 Caller に当たる Caller の、権限の上限と同じかより低い水準だけの要求は、要求した水準で発行する。それ以外の要求は、リポジトリポリシーが許す対象を混ぜていても全体を認可拒否とし、利用者に返すトークンを取得しない。リポジトリポリシーを変えた後の要求は、変えた後の内容で判定して認可拒否とする。シンボリックリンクを置いた対象は、リンク先のファイルの内容で判定して発行する。拒否の理由は監査記録に残り、応答と監査記録にリポジトリポリシーの内容が出ない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-011` | Functional Requirement |
| `alignment` | `FR-AUTHZ-004` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-002` | Functional Requirement |
| `alignment` | `FR-OPS-003` | Functional Requirement |

### AC-AUTHZ-015 有効なリポジトリポリシーのない対象を含む要求の拒否

**条件**：リポジトリポリシーを適用するデプロイで、対象の固定のパスに次のものを置き、ファイルに誤りがなければリポジトリポリシーが許す Caller から要求する。書式の誤ったファイル、空のファイル、許可 Caller の Repository ID かリポジトリ名が重複したファイル、空の権限の上限を持つ許可 Caller を含むファイル、GATES が許可しない権限名と水準の組を含む権限の上限を持つ許可 Caller を含むファイル、大きさの上限を超えるファイル、ディレクトリ、存在しないファイルを指すシンボリックリンク。空の権限の上限を持つ許可 Caller を含むファイルと、許可しない組を含む権限の上限を持つ許可 Caller を含むファイルでは、ほかの許可 Caller を正しく書く

**期待する結果**：どの要求も全体を認可拒否とし、正しい許可 Caller の部分だけを使って発行しない。利用者に返すトークンを取得しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-011` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-002` | Functional Requirement |

### AC-AUTHZ-016 保護対象への要求での、保護対象の一覧とリポジトリポリシーの両方による判定

**条件**：リポジトリポリシーを適用し、空でない保護対象の一覧でデプロイして、保護対象に次の要求をする。保護対象の一覧とリポジトリポリシーの両方が許す Caller と権限で要求する。保護対象の一覧だけが許す Caller か権限で要求する。リポジトリポリシーだけが許す Caller か権限で要求する

**期待する結果**：両方が許す要求だけを、要求した水準で発行する。片方だけが許す要求は全体を認可拒否とし、利用者に返すトークンを取得しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-011` | Functional Requirement |
| `alignment` | `FR-AUTHZ-009` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-002` | Functional Requirement |

### AC-AUTHZ-017 リポジトリポリシーの適用の設定

**条件**：リポジトリポリシーを適用しない設定でデプロイし、リポジトリポリシーのない対象、不正なリポジトリポリシーの対象、Caller を許していないリポジトリポリシーの対象を要求する。また、リポジトリポリシーの適用が欠落した設定か、適用するとしないのどちらでもない値の設定でデプロイし、発行を要求する

**期待する結果**：適用しないデプロイでは、リポジトリポリシーを読まず、ポリシー読み取りトークンを取得せず、どの要求も、リポジトリポリシーの有無と内容によらず、AC-AUTHZ-014 から AC-AUTHZ-016、AC-ISSUANCE-011、AC-ISSUANCE-012 を除く AC の期待する結果のとおりになる。欠落か不正な設定のデプロイでは、どの要求にもトークンを返さず、設定の誤りは、値を記録せずに、どの設定の誤りかを運用者が特定できる形で残る

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-OPS-001` | Functional Requirement |
| `alignment` | `FR-AUTHZ-011` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-002` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-007` | Functional Requirement |
| `alignment` | `QR-003` | Quality Requirement |

## Token Issuance

### AC-ISSUANCE-001 許可オーナーの正当な要求に対するトークンの発行

**条件**：許可されたオーナーのワークフローが、正当な GATES 用 OIDC JWT で、GATES が許可する権限のうち GitHub App に付与された権限を一つ要求

**期待する結果**：指定対象だけの短命トークンを取得できる。応答に有効期限、対象、実効権限が含まれ、秘密鍵と内部 GitHub App JWT を含まない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHN-001` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-002` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-005` | Functional Requirement |

### AC-ISSUANCE-002 同一 Installation の複数対象への発行

**条件**：同一 Installation の複数対象を、GATES が許可する権限のうち GitHub App に付与された権限で要求

**期待する結果**：指定した複数対象だけへのトークンを発行

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-ISSUANCE-003` | Functional Requirement |

### AC-ISSUANCE-003 App と Installation の範囲が要求より広い場合の発行範囲の限定

**条件**：App の権限上限・Installation の対象が要求より広い

**期待する結果**：発行結果は要求した対象と権限だけに限定（FR-ISSUANCE-004 の除外対象を除く）

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-ISSUANCE-003` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-004` | Functional Requirement |

### AC-ISSUANCE-004 照合に失敗した発行応答でのトークンの不返却と失効の試行

**条件**：GitHub API の発行応答が対象・権限（FR-ISSUANCE-004 の例外を除く）の範囲外、不変 Owner ID 不一致、必要な対象一覧の欠落、または不整合

**期待する結果**：トークンを返さない。空でないトークン文字列を取得した場合は失効 API を試み、成否を監査する。失効成功は保証しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-005` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-004` | Functional Requirement |

### AC-ISSUANCE-005 GitHub App Private Key の更新後の発行

**条件**：署名に用いる GitHub App Private Key の新しい鍵への切り替えと、GitHub 側の旧鍵失効

**期待する結果**：新しい鍵を加えても旧鍵での署名と発行が続き、切り替えの後と旧鍵の失効の後も署名と発行が成立

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-ISSUANCE-001` | Functional Requirement |
| `alignment` | `FR-OPS-005` | Functional Requirement |

### AC-ISSUANCE-006 発行 API の応答を受信できない場合の扱い

**条件**：GitHub 発行 API に要求が到達した後、応答の受信に失敗してトークン文字列を取得できない

**期待する結果**：トークンを返さず、GitHub 側の発行有無・失効成功を保証しない。取得できないトークンに対する失効は要求しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-ISSUANCE-004` | Functional Requirement |
| `alignment` | `QR-003` | Quality Requirement |

### AC-ISSUANCE-007 ジョブの終了時のトークンの失効

**条件**：利用者接点がトークンを後続ステップへ渡したジョブが終了する

**期待する結果**：ジョブの終了時に利用者接点がそのトークンの失効を試み、失効の成否をワークフローのログに出し、トークンをログに出さない。失効が失敗してもジョブの結果は変わらない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-ISSUANCE-006` | Functional Requirement |
| `alignment` | `QR-004` | Quality Requirement |

### AC-ISSUANCE-008 後続ステップへ渡す前に失敗した場合のトークンの失効

**条件**：利用者接点がトークンを受け取った後、成功応答の判定か、後続ステップとジョブの終了時の処理へのトークンの受け渡しに失敗する

**期待する結果**：利用者接点がその時点でトークンの失効を試み、失効の成否をワークフローのログに出し、トークンをログに出さずにエラー終了する

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-ISSUANCE-006` | Functional Requirement |
| `alignment` | `QR-004` | Quality Requirement |

### AC-ISSUANCE-009 ジョブのキャンセル時のトークンの失効

**条件**：利用者接点が GATES の呼び出しを始めた後、トークンを後続ステップへ渡す処理を終える前にジョブがキャンセルされる

**期待する結果**：利用者接点はトークンを受け取った場合、後続ステップへ渡す前ならその時点で、渡した後ならジョブの終了時に、トークンの失効を試み、トークンをログに出さない。Runner が利用者接点を強制終了した場合は失効の試行を保証しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-ISSUANCE-006` | Functional Requirement |
| `alignment` | `QR-004` | Quality Requirement |

### AC-ISSUANCE-010 複数の権限と読み取り以外の水準の発行

**条件**：GATES が許可する権限のうち GitHub App に付与された権限から、複数の権限を、読み取り以外の水準を含めて要求する

**期待する結果**：要求した権限名と水準の組だけを持つトークンを取得できる（FR-ISSUANCE-004 の除外対象を除く）。そのトークンで、要求した権限が許す操作は成功し、GitHub App に付与されていても要求しなかった権限の操作は拒否される

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-006` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-003` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-004` | Functional Requirement |

### AC-ISSUANCE-011 ポリシー読み取りトークンの取得の限定、非返却、失効

**条件**：リポジトリポリシーを適用するデプロイで、次の要求をする。ポリシー読み取りトークンを発行する前に判定する、リポジトリポリシーの条件のほかの認可条件で拒否される要求。発行に成功する要求。リポジトリポリシーで認可拒否になる要求。リポジトリポリシーの取得の一時的な障害が起きる要求。ポリシー読み取りトークンの発行応答が照合に失敗する要求（権限、対象、読めるファイルのパスが一致しない）

**期待する結果**：ポリシー読み取りトークンを発行する前に判定する認可条件で拒否される要求では、ポリシー読み取りトークンを取得しない。それ以外の要求では、ポリシー読み取りトークンは `single_file:read` と付随権限だけを持ち、応答、ワークフローのログ、監査記録に出ない。取得できたポリシー読み取りトークンは失効が試みられ、その成否が、利用者に返すトークンの照合失敗での失効の結果とは別の項目として監査記録に残る。失効成功は保証しない。発行応答が照合に失敗したポリシー読み取りトークンは、リポジトリポリシーの取得に使われず、照合失敗として扱われる

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-ISSUANCE-007` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-002` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-004` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-005` | Functional Requirement |
| `alignment` | `FR-OPS-003` | Functional Requirement |
| `alignment` | `QR-004` | Quality Requirement |

### AC-ISSUANCE-012 リポジトリポリシーを読んだリポジトリと Repository ID が一致しない発行結果の不返却

**条件**：リポジトリポリシーを適用するデプロイで、利用者に返すトークンの発行応答の実効対象の Repository ID が、同じ完全名でリポジトリポリシーを読んだリポジトリ（ポリシー読み取りトークンの実効対象）の Repository ID と一致しない状態を作る。リポジトリポリシーを読んだ後に対象を削除して同じ名前で作り直した場合に当たる

**期待する結果**：トークンを返さない。空でないトークン文字列を取得した場合は失効 API を試み、成否を監査する。失効成功は保証しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-AUTHZ-012` | Functional Requirement |
| `alignment` | `FR-ISSUANCE-004` | Functional Requirement |

## Usability and Operations

### AC-OPS-001 対象、権限、接続情報だけを利用者接点に渡したときのトークンの取得

**条件**：GitHub がホストする Runner で、利用者が `id-token: write` を宣言し、利用者接点に対象と権限、および運用者が提供する接続情報だけを渡す

**期待する結果**：他の手順や設定なしでトークンを取得できる

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-OPS-006` | Functional Requirement |

### AC-OPS-002 利用者接点が発行処理のエラーを受け取ったときの終了

**条件**：利用者接点が発行処理のエラーを受け取る

**期待する結果**：エラー終了し、JWT・トークンをログへ出さない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-OPS-006` | Functional Requirement |
| `alignment` | `QR-004` | Quality Requirement |

### AC-OPS-003 失敗の種別ごとに区別したエラー

**条件**：入力不正、認証失敗、認可拒否、Installation 利用不可、GitHub 発行 API の `422`、発行結果の照合失敗、外部 API・KMS の一時的な障害をそれぞれ発生させる

**期待する結果**：種別ごとに区別したエラーを返す

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-OPS-004` | Functional Requirement |

### AC-OPS-004 成功と認証後の拒否の監査記録

**条件**：発行の成功と、認証後の拒否（認可拒否、発行結果の照合失敗など）をそれぞれ実行する

**期待する結果**：監査記録に Caller（Owner ID、Repository ID、ワークフロー実行 ID、試行番号、JWT に含まれていたワークフロー起動属性）、対象、権限、結果、相関 ID が残り、資格情報を含まない。成功応答にも相関 ID が含まれる

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-ISSUANCE-005` | Functional Requirement |
| `alignment` | `FR-OPS-003` | Functional Requirement |

### AC-OPS-005 構築した AWS リソースでの Lambda の実行 Role の権限の限定、Invoke を許可する主体の限定、認証なしの呼び出し口の不在

**条件**：GATES の AWS リソースを構築・変更

**期待する結果**：Lambda の実行 Role が KMS の署名操作と必要なログ出力だけを許可され、鍵のインポート・置換・削除・管理設定の変更は許可されない。GATES の構成が Lambda の Invoke を許可するのは、GATES の呼び出しに使う IAM Role だけで、その許可は GATES の関数の Invoke に限られる。認証なしの呼び出し口が存在しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-OPS-002` | Functional Requirement |
| `alignment` | `QR-008` | Quality Requirement |

### AC-OPS-006 秘密鍵のインポートとインポート前のコピーの削除

**条件**：秘密鍵を独立した運用で KMS にインポートし、インポート前のコピーを削除する

**期待する結果**：インフラの構成管理の状態と計画、リポジトリ、CI の成果物に鍵素材を残さず、KMS 署名が成立する。インポート前のコピーが運用者の手元に残っていない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-ISSUANCE-001` | Functional Requirement |
| `alignment` | `QR-008` | Quality Requirement |

### AC-OPS-007 Invoke を許可されていない主体による Lambda の呼び出しの拒否

**条件**：AWS の認証情報を持たない、または Invoke を許可されていない主体が Lambda を呼び出す

**期待する結果**：関数の実行前に拒否される

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-OPS-002` | Functional Requirement |

### AC-OPS-008 発行 API の `422` の GitHub 発行拒否としての扱い

**条件**：発行 API が `422` を返す

**期待する結果**：GATES 自身の認可拒否と区別した GitHub 発行拒否として扱い、トークンを返さない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-OPS-004` | Functional Requirement |
| `alignment` | `QR-003` | Quality Requirement |

### AC-OPS-009 署名検証を通った Claims Set の調査用の記録

**条件**：発行の成功、署名検証の後の Claim の検証による認証失敗、署名検証による認証失敗をそれぞれ実行する。JWT には未知の Claim と、制御文字を含む文字列の Claim を含める

**期待する結果**：署名検証を通った 2 つの場合は、監査記録に未知の Claim を含む Claims Set の全体が残り、制御文字はエスケープされて記録は 1 行に収まる。署名検証で拒否した場合は Claims Set が残らない。どの場合も JWT の文字列は残らず、Claims Set は応答に含まれない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-OPS-003` | Functional Requirement |
| `alignment` | `QR-004` | Quality Requirement |

### AC-OPS-010 内部実装の変更に対する利用者のワークフローの維持

**条件**：GATES の認証・発行方式の内部実装を変更

**期待する結果**：利用者のワークフロー定義を変更せずに、AC-OPS-001 の条件でトークンを取得できる

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-OPS-006` | Functional Requirement |

## Quality and Security

### AC-QUALITY-001 OIDC 公開鍵、KMS、GitHub API の障害での Fail Closed

**条件**：OIDC 公開鍵・KMS・GitHub API の取得・通信障害。リポジトリポリシーを適用するデプロイでの、リポジトリポリシーの取得の一時的な障害（通信の失敗、タイムアウト、サーバー側のエラー、レート制限）を含む

**期待する結果**：Fail Closed。資格情報を露出しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `QR-003` | Quality Requirement |
| `alignment` | `FR-AUTHZ-011` | Functional Requirement |

### AC-QUALITY-002 ログとエラー本文からの秘密情報の除外

**条件**：正常系と異常系を実行する

**期待する結果**：発行処理と利用者接点のログとエラー本文に、秘密鍵・OIDC JWT・GitHub App JWT・トークン・AWS 一時認証情報が含まれない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `QR-004` | Quality Requirement |

### AC-QUALITY-003 GitHub App Private Key の平文の不在

**条件**：GitHub Secrets、Actions Runner、Lambda の設定と実行環境を確認

**期待する結果**：GitHub App Private Key の平文が、いずれにも存在しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `QR-008` | Quality Requirement |
