# Traceability Policy

要求や受理条件のように、ほかのドキュメント、コード、`formal/` から個別に参照される種類の定義には ID を付けます。
参照元は ID で定義を指すため、ID と意味の対応が崩れると、参照元の記述は変わらないまま、別の意味を指すようになります。
本ドキュメントは、ID を付ける定義の種類、ID の振り方と変え方、定義と参照の書き方、外部仕様の条項の書式、GOJ と条項の関係の書き方、テストと `formal/` から定義を参照する注釈を定めます。

## 1. ID Kinds

ID を付ける種類は次のとおりです。

| 接頭辞 | 対象 | 定めるドキュメント | ID の形 |
|---|---|---|---|
| `FR-` | 機能要求 | `requirements.md` | カテゴリのスラッグと、カテゴリごとの 3 桁の連番（`FR-AUTHN-002`）。スラッグは `AUTHN`（Authentication）、`AUTHZ`（Authorization）、`ISSUANCE`（Token Issuance）、`OPS`（Usability and Operations） |
| `QR-` | 品質とセキュリティの要求 | `requirements.md` | 3 桁の連番（`QR-003`） |
| `AC-` | 受け入れ条件 | `acceptance-criteria.md` | カテゴリのスラッグと、カテゴリごとの 3 桁の連番（`AC-AUTHZ-001`）。スラッグは `FR-` の 4 つに `QUALITY`（Quality and Security）を加えた 5 つ。`FR-` を判定する AC は、期待する結果が判定する `FR-` のカテゴリに置き、`QR-` だけを判定する AC は `QUALITY` に置く |
| `GOJ-` | GATES 用 OIDC JWT の受理条件 | `gates-oidc-jwt-claims.md`、`gates-oidc-jwt-signature.md` | Claim 名か、規則の内容を表す名前。作り方は `gates-oidc-jwt-claims.md` に定める |
| `AOJ-` | AWS 用 OIDC JWT の Claim に課す条件 | `aws-oidc-jwt.md` | Claim 名か、条件の内容を表す名前 |
| `GAJ-` | GitHub App JWT の生成条件 | `github-app-jwt.md` | Claim 名か、条件の内容を表す名前 |
| `RULE-` | ドメインの規則 | `domain.md` | 3 桁の連番（`RULE-001`） |
| `INV-` | ドメインの不変条件 | `domain.md` | 3 桁の連番（`INV-001`） |
| `RFC7515-`、`RFC7517-`、`RFC7518-`、`RFC7797-`、`RFC9964-` | JWS、JWK、JWA、JWS Unencoded Payload Option、ML-DSA の条項 | `docs/external/standards/jose.md` | 条項の内容を表す名前 |
| `RFC7159-`、`RFC7519-`、`RFC8725-` | JSON、JWT、JWT Best Current Practices の条項 | `docs/external/standards/jwt.md` | 条項の内容を表す名前（`RFC7519-AUD-REJECT`） |
| `OIDCC-`、`OIDCD-` | OpenID Connect Core、OpenID Connect Discovery の条項 | `docs/external/standards/oidc.md` | 条項の内容を表す名前 |
| `GH-OIDC-` | GitHub Actions OIDC の条項 | `docs/external/github/oidc.md` | 条項の内容を表す名前 |
| `GH-ACCOUNT-` | GitHub のオーナーとリポジトリの条項 | `docs/external/github/account.md` | 条項の内容を表す名前 |
| `GH-APP-` | GitHub App の条項 | `docs/external/github/app.md` | 条項の内容を表す名前 |
| `GH-ACTIONS-` | GitHub Actions の連携の条項 | `docs/external/github/actions.md` | 条項の内容を表す名前 |

これらの種類の定義には、重要度にかかわらず、すべて ID を付けます。
一部の定義だけに ID がないと、その定義を参照元から指せず、付与漏れも機械的に確かめられないためです。

外部仕様の条項（[§5](#5-external-specification-clauses)）の ID は、出典の資料を表す接頭辞と、条項の内容を表す名前でできています。

- 接頭辞は出典の資料を表す。ID を見ただけで、条項を確かめるときに開く資料が決まるようにするためである
- 名前は英大文字、数字、`-` で書き、規範が扱う要素（Claim 名、Header Parameter 名など）を先に、規範の内容を後に置く（`RFC7519-AUD-REJECT`）。同じ要素の条項が ID の順に並び、grep で集められるようにするためである
- 名前に節番号を使わない。一つの節が複数の規範を定めることがあり（RFC 7519 §4.1.3 は `aud` の形式と拒否を定める）、GitHub の資料には節番号がないためである
- GitHub の条項の ID に Evidence を含めない。Unconfirmed だった事柄が公式資料で確かめられて Documented になっても、条項が指す事柄は変わらないためである

## 2. Assigning and Changing IDs

ID は定義の意味を指し、定義の書き方や置き場所を指しません。

- 見出しのタイトルの変更、ファイルの移動、ファイルの中での並び替えでは ID を変えない
- `FR-` と `AC-` の ID はカテゴリを含むため、定義を別のカテゴリへ移すときは ID を変える。この変更は、後に定める振り直しの規則に従って行う
- 連番の ID では、新しい定義に、種類ごと（`FR-` と `AC-` はカテゴリごと）にこれまでに振った最大の番号の次を振る。削除した定義の番号を新しい定義に振らない。古い意味のまま残った参照が、知らないうちに新しい意味を指すためである
- 定義を分割または統合したときは、できた定義のすべてに新しい ID を振り、旧 ID は使わない。旧 ID の参照元は一括で置換せず、参照元ごとに、どの定義が旧 ID の意味を引き継ぐかを判断して付け替える
- ID を変えずに意味を変えたときは、その ID を参照する設計、`formal/`、コード、テストが新しい意味に合うかを確かめる
- 一つの ID は一つの判定を表す。受理条件と生成条件（`GOJ-`、`AOJ-`、`GAJ-`）の定義に、関係する外部仕様の条項の組、`formal/` の述語、検査のいずれかが異なる複数の判定を含めない（`sub` の形式の判定と、`sub` の長さの上限の判定は別の ID にする）。一つの判定が複数の条項に関係することはあってよい。一つの ID に複数の判定を含めると、関係（[§6](#6-relations-between-goj-and-clauses)）や注釈（[§7](#7-trace-annotations)）が ID 全体を指すことになり、どの判定を確かめたのかが ID から決まらないためである

これらの規則では、新しい定義の番号はファイルの中の位置にかかわらず末尾に続き、削除した番号は欠けたまま残るため、運用を続けると、番号の欠けや、番号の順とファイルの中の並び順が一致しない定義が増えます。
コードのリファクタリングと同じく、これらは番号を振り直して時々整理します。
振り直しは、リポジトリ内の参照の一貫性を保てる場合に限り、次の規則で行います。

- 振り直しは、定義の意味を変える変更と分けた単独の変更にする。意味の変更が番号の置き換えの差分に紛れると、レビューで見落とすためである
- 旧 ID から新 ID への対応表を作り、ドキュメント、`formal/`、コード、テストの参照を、同じ変更で対応表から一度にすべて付け替える。一件ずつ順に置換すると、付け替えた ID がさらに別の ID に置き換わることがあるためである
- 差分が対応表どおりの ID の置き換えだけであることを確かめる
- 振り直しでは、削除した番号を詰めてよい。振り直した後の新しい定義には、振り直した後の最大の番号の次を振る
- 対応表をコミットメッセージに残す。過去のコミット、Issue、Pull Request の記述は旧 ID のまま残るためである

## 3. Writing Definitions

- 定義ごとに見出しを置き、見出しは ID、半角スペース、タイトルの順に書く（例：`#### FR-AUTHZ-001 許可オーナーの Owner ID による認可`）
- 定義を並べる節の直下の見出しは、定義の見出しだけにする
- 文書の直下は、定義を並べる節に当たらない。定義の見出しを、定義でない節の見出しと同じ階層に並べてよい（例：`github-app-jwt.md` の `## GAJ-ISS` と `## Traceability`）。定義でない節は見出しで区切られ、直前の定義の本文として読まれないためである
- 定義を分類するときは、分類ごとに節を設け、その節の直下に定義の見出しを並べる（例：`requirements.md` の `### 3.2 Authorization` の直下に `#### FR-AUTHZ-001 …`）
- 定義を並べる節の説明は、最初の定義の見出しより前に書く。最後の定義の後に書くと、その定義の本文として読まれるためである
- 定義を一覧する表を索引として置いてもよい。その場合も、定義の見出しとその下の本文を正とする

## 4. Referring to Definitions

- ほかのファイルの定義を参照するときは、本文に ID を書き、リンクはファイル単位にする（例：`[Requirements](../spec/requirements.md) の FR-AUTHZ-001`）。見出しのアンカーは見出しの文字列から作られ、タイトルを変えると切れるためである
- 同じファイルの中では、見出しのアンカーでリンクしてよい。タイトルを変えた差分の中で、切れたリンクを確かめられるためである
- 地の文、表、インラインコードに書く ID は、定義された ID に限る。まだ定義されていない ID を書式の例に使うときは、フェンスで囲んだコードブロックの中に書く。追跡の検査は、コードブロックの外に現れる ID がすべて定義されていることを確かめ、コードブロックの中は例として扱うためである
- コード、テスト、`formal/` からは、文書名を添えず ID だけで定義を参照する。ID の接頭辞から定めるドキュメントが決まり（[§1](#1-id-kinds)）、grep で定義を探せるためである
- 定義の一部を、番号付きの条件（「GOJ-SUB の条件 6」）のように定義の中の位置で参照しない。条件を足したり並べ替えたりすると、同じ番号が別の条件を指すようになるためである。一部だけを参照する必要があるなら、その部分を別の ID の定義に分ける（[§2](#2-assigning-and-changing-ids)）

## 5. External Specification Clauses

`docs/external/` は、外部仕様の規定を、ID を付けた **条項** に分けて書きます。
GOJ と `formal/` のモデルが、外部仕様の規定を一つずつ ID で指せるようにするためです。
一つの条項は、一つの規範だけを持ち、標準仕様の条項では一つの Level と一つの Actor、GitHub の条項では一つの Evidence だけを持ちます。

- 「A は MUST、B は SHOULD」という一文は、2 つの条項に分ける
- 複数の主体を並べて義務を課す文（RFC 8725 §3.7 の「Implementations and applications MUST」）は、主体ごとの条項に分ける。一つの主体を複数の呼び方で示す文（RFC 8725 §3.9 の「the relying party or application」）は分けず、一つの呼び方を Actor に書き、根拠を本文に書く
- 一つの文の中でも、主体の異なる義務（RFC 8725 §3.1 の、鍵を一つのアルゴリズムだけで使う義務と、それを暗号操作の時点で確かめる義務）は、条項を分ける
- 同じ規範を言い換えた複数の文（RFC 7519 §4.1.4 の、`exp` の時刻以後の受理を MUST NOT とする文と、現在時刻が `exp` より前であることを MUST とする文）は、一つの条項にまとめる。言い換えた文の RFC 2119 の語が異なるときは、最初の文の語を Level に書く
- 同じ主体に、値の検証と、検証に失敗したときの拒否を求める文（RFC 8725 §3.8 の、鍵が Issuer に属することを検証する MUST と、属さなければ拒否する MUST）は、一つの受理の判定を表すため、一つの条項にまとめる。検証の義務は、拒否と切り離して確かめられる効果を持たないためである
- RFC 2119 の語を含まず、Definition（後述）にも当たらない説明の文は、条項にしない。関係する条項の本文か、ファイルの冒頭の対象の説明に書く

条項の項目名、Level と Evidence の値、関係の表の列名と Relation の値（[§6](#6-relations-between-goj-and-clauses)）は英語で書きます。
これらの語は、本ドキュメントが定める意味だけを持ちます。
英語で書くのは、日本語の文書の中で、同じ意味の普通の語（「定義」「推論」「満たす」など）と見分けられるようにするためです。
地の文でこれらの意味を指すときも、訳さず、バッククォートで囲まずに英語のまま書きます。

条項の見出しは [§3](#3-writing-definitions) のとおり ID とタイトルで書き、見出しの直下に、次に定める項目の箇条書きを置きます。
箇条書きの後の本文には、原文で読み取りにくい点の説明だけを書きます。
ほかの条項を緩めたり限定したりする条項（`exp` の判定に猶予を認める MAY など）は、緩める相手の条項の ID を本文に書きます。
条項は一つずつ独立に読まれるため、書かないと、緩められる側の条項だけを読んだ読者やモデルが、標準仕様より厳しい規範として扱うためです。
GATES がその規定をどう扱うかは、条項には書かず、GOJ の側に書きます（[§6](#6-relations-between-goj-and-clauses)）。

### Standard Clauses

標準仕様の条項は、次の形で書きます。

```markdown
#### RFC7519-AUD-REJECT 受信者を含まない aud の拒否

- **Source**：[RFC 7519 §4.1.3](https://www.rfc-editor.org/rfc/rfc7519.html#section-4.1.3)
- **Quote**：`If the principal processing the claim does not identify itself with a value in the "aud" claim when this claim is present, then the JWT MUST be rejected.`
- **Level**：MUST
- **Actor**：Recipient
- **Condition**：`aud` が存在する
```

- **Source**：資料、節、節へのリンク
- **Quote**：条項の根拠となる一文から数文の逐語の引用。文の単位で、一続きの範囲を引用し、途中を省略しない。一つの文が複数の条項に分かれるときは、それぞれの条項に同じ文を引用する。RFC のテキスト版の改行、ページの区切り、行頭の字下げは除いて一行につなぎ、それ以外の文字は変えない。要約や訳を原文の代わりにしない。条項を出典と照合するレビューで、写した Level、Actor、Condition を原文と突き合わせられるようにするためである
- **Level**：[RFC 2119](https://www.rfc-editor.org/rfc/rfc2119.html) の語（MUST、MUST NOT、REQUIRED、SHALL、SHALL NOT、SHOULD、SHOULD NOT、RECOMMENDED、NOT RECOMMENDED、MAY、OPTIONAL）のうち原文が使う語か、Definition。Definition は、NumericDate の形式や `aud` の値の形のように、RFC 2119 の語を使わず義務も課さないが、`formal/` のモデルが前提とする規定に使う
- **Actor**：規範に従う主体。出典の資料が主体に使う英語の名前を、訳さずに、単数形で先頭を大文字にして書く（Issuer、Recipient、Implementer、Library、Application、OpenID Provider、Relying Party など）。OP や RP のような略語は使わず、資料が定める正式な名前で書く。訳語や略語は、読み手によって指す主体の解釈が揺れるためである。原文が主体を書かない規範（受動態の文など）では、その規範に従って処理する主体を、同じ資料が使う名前で書き、主体を決めた根拠を本文に書く。Level が Definition の条項では省く
- **Condition**：原文が書く、規範が適用される条件（ID Token に限る、Authorization Code Flow に限る、`aud` が存在する場合に限る、など）。原文に条件がなければ省く。原文が書かず、ほかの条項から導かれる適用範囲（Claim の使用が OPTIONAL なので、Claim がない JWT には適用されない、など）は、Condition に書かず本文に書く。Condition を原文と突き合わせられるようにするためである

### GitHub Clauses

GitHub の仕様の条項は、次の形で書きます。

```markdown
#### GH-OIDC-AUD-DEFAULT Audience の既定値

- **Evidence**：Documented
- **Source**：[OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)
- **Statement**：Audience を指定しない場合、`aud` はリポジトリのオーナーの URL である
```

- **Evidence**：次の表のいずれか
- **Source**：GitHub の公式資料へのリンク、または `docs/research/github/` の調査記録へのリンク
- **Statement**：GitHub の振る舞い。Observed の条項では、記録した条件（Audience の指定の有無、リポジトリのオーナーの種別など）を含める。Inferred の条項では、根拠にした条項の ID を含める
- **Confirmation**：Unconfirmed の条項だけに書く。何を確かめれば確定するか（公式資料の記述、実測する条件など）

Evidence の値は、Statement がどのように知られたかを表すため、すべて過去分詞にそろえます。

| Evidence | 意味 |
|---|---|
| Documented | リンクした GitHub の公式資料に記述された内容。資料の条件・適用範囲を含めて読む |
| Published | GitHub が公開するエンドポイントから取得し、調査記録に残した値。取得時点の値であり、将来も同じ値であることの保証ではない |
| Observed | GitHub Actions の実環境で発行された JWT、GitHub API の実際の応答、GitHub Actions の実行ログから取得し、調査記録に残した値。記録した条件での観測であり、ほかの条件や将来も同じ値であることの保証ではない |
| Inferred | Documented、Published、Observed の条項から導いた解釈。GitHub が明示的に保証した事実とは区別する |
| Unconfirmed | 公式資料で確定できない動作。Published や Observed の値があっても、それだけでは保証とみなさない |

Unconfirmed の条項は、「GitHub は X を定めていない」「X の条件は資料から確定できない」の形で、確定していない事柄を書きます。
確定していない振る舞いを断定の形で書くと、GOJ や `formal/` のモデルが、それを GitHub の保証として扱うおそれがあるためです。

## 6. Relations between GOJ and Clauses

GOJ の定義には、外部仕様の条項との **関係** を、見出しの直下に表で書きます。
関係は GOJ の側にだけ書き、条項の側にはどの GOJ と関係するかを書きません。
`docs/external/` は `docs/design/` に依存しないためです（[Documentation Policy](docs-policy.md) の §2.9）。

```markdown
### GOJ-AUD `aud`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7519-AUD-REJECT | なし |
| Narrows | RFC7519-AUD-FORMAT | 要素が複数の配列を拒否する。受信者を増やした JWT を資格情報として再利用させないため |
| Accepts | GH-OIDC-AUD-CUSTOM | なし |
```

- **Relation**：関係の種類。後に定める 5 つのいずれか
- **Clause**：条項の ID。1 行に一つだけ書く
- **Note**：関係の補足。補足がなければ「なし」と書く。空欄にしないのは、書き忘れと、補足がないことを区別するためである

1 行には、一つの関係の種類と一つの条項の組だけを書きます。
一つの条項を複数の種類に挙げるときも、行を分けます（`exp` を JSON の整数に限る GOJ は、NumericDate の形式の条項を、受理した値が満たす点で Satisfies の行に、整数でない値を拒否する点で Narrows の行に挙げる）。
行は、Relation を Satisfies、Narrows、Accepts、Possibly Rejects の順に並べ、同じ Relation の中では Clause の ID の順に並べます。
関係する条項がない GOJ には、Relation が None、Clause が「なし」の 1 行だけを書きます。
表を省かないのは、関係を書き忘れた GOJ を機械的に見つけられるようにするためです。

Relation の値は次の 5 つに限ります。
Satisfies と Narrows は標準仕様の条項に、Accepts と Possibly Rejects は GitHub の条項に使います。

- **Satisfies**：GOJ に従って受理と拒否を決めれば、標準仕様の条項に従ったことになる。Actor が Recipient のように受信の側の条項なら、GOJ が受理する JWT は条項の下で受理してよい JWT に限られ、Actor が Issuer のように発行の側の条項や、Level が Definition の条項なら、GOJ が受理する JWT は条項を満たす。`formal/` の安全性の検査の対象になる
- **Narrows**：標準仕様の条項が許す入力の一部を、GOJ が拒否する。Level が MAY や OPTIONAL の条項が許す選択（Claim を省くことなど）や、Level が Definition の条項が表す値の形の一部を拒否する場合も含む。互換性に影響するため、狭める理由を Note に書く
- **Accepts**：GitHub の条項が表す発行を、GOJ が受理する。`formal/` の互換性の検査の対象になる
- **Possibly Rejects**：GitHub の条項（多くは Evidence が Unconfirmed の条項）が許す発行の一部を、GOJ が拒否する。GitHub がその発行を実際に行うかは確定していないため、拒否は起こり得るものにとどまる。GATES が受け入れた既知の非互換を表し、互換性の検査の反例に対応する。拒否し得る発行と、それを受け入れた理由を Note に書く
- **None**：関係する条項がない

GATES の役割に当たる強い規範の条項は、いずれかの GOJ の Satisfies か Narrows の行で参照するか、「適用しない標準の条項」の表に挙げます。
強い規範とは、Level が MUST、MUST NOT、REQUIRED、SHALL、SHALL NOT の条項です。
それ以外の Level の条項は、標準仕様が従わない選択を許すか、義務を課さないため、網羅を求めません（関係を書くことは妨げません）。

- **GATES の役割に当たる Actor の一覧**：Recipient、Relying Party のように、GATES が当たる条項の Actor を挙げる。`gates-oidc-jwt-claims.md` に置き、`gates-oidc-jwt-signature.md` も同じ一覧に従う
- **適用しない標準の条項**：GATES の役割に当たる強い規範のうち、GATES に適用しない条項の ID と、適用しない理由（`nonce` の照合は認証要求を前提とする、など）を並べた表。`gates-oidc-jwt-claims.md` と `gates-oidc-jwt-signature.md` のそれぞれに置き、条項が扱う対象が Claims Set か、直列化、JOSE Header、Discovery、JWK かによって、[Documentation Policy](docs-policy.md) の §2.3 に定める責務に合う側に挙げる

## 7. Trace Annotations

テストと `formal/` は、検査や形式化の対象とする定義を、行コメントの **注釈** で ID によって宣言します。
対応を注釈の側だけに書くのは、`docs/` のドキュメントにテストの名前、`formal/` の述語の名前、検査のファイル名を書くと、名前を変えたり分けたりしたときにドキュメントが古くなるためです。
注釈はテスト、定義、検査と同じファイルの決まった位置に置くため、名前を変えたり分けたりしても、注釈は対象とともに移ります。
定義の側から対応を探すときは、注釈を grep します。

- 注釈は、`trace:checks` か `trace:formalizes` の後に、ID を半角スペース区切りで書く（`trace:unformalized` は後述の形で書く）
- 注釈は対応だけを表す。検査の合格や、定義の条件を網羅したことは表さない
- `docs/` のドキュメントには、テストの名前、`formal/` の述語の名前、検査のファイル名を書かない

### Tests

AC の判定の証拠にするテストには、そのテストが検査対象とする AC を `trace:checks` で宣言します。
AC の意味を変えたときに確かめ直すテスト（[§2](#2-assigning-and-changing-ids)）を、宣言の grep で探せるためです。

- 宣言は、テストの直前の行コメントに書く。Go のテスト関数では `//trace:checks AC-AUTHN-002 AC-AUTHZ-001`、Terraform の試験の `run` ブロックでは `# trace:checks AC-OPS-005` とする
- Go では `//` の直後に空白を置かない。Go はこの形の行をディレクティブとして扱い、doc comment の本文に含めないためである。doc comment を持つテスト関数では、本文の後に空行の `//` を挟んで最後の行に置く。gofmt が整える形である
- 宣言は、AC の判定の証拠にするテストだけに付ける。すべてのテストに付けることは求めない
- AC の判定の記録には、証拠にしたテストの名前を転記しない。テストは `grep -rnE 'trace:checks.* AC-AUTHN-002( |$)'` で探す

### Formal Models

`formal/` のモデルは、外部仕様の条項と GOJ を、`trace:formalizes`、`trace:checks`、`trace:unformalized` で参照します。
コメントの記号は、SMT-LIB では `;`、Alloy では `//` とします。

```smt2
; trace:formalizes RFC7519-AUD-REJECT
(define-fun std-aud-reject ...)
```

```smt2
; expect: unsat
; trace:checks GOJ-AUD RFC7519-AUD-REJECT
```

- **`trace:formalizes`**：直後の定義（SMT-LIB の `define-fun`、Alloy の `pred`、`fact`、`fun`）が、挙げた条項または GOJ の形式化であることを表す。定義の直前の行に置く
- **`trace:checks`**：その検査が、挙げた ID の定義、または ID どうしの関係を確かめることを表す。Z3 の検査は一つのファイルが一つの検査なので、1 行目の `; expect:` の次の行に置く。Alloy の検査は、`check` か `run` のコマンドの直前の行に置く。GOJ と条項の ID を挙げた検査は、挙げた GOJ と、挙げた各条項との関係を確かめるものとして読む
- **`trace:unformalized`**：挙げた条項または GOJ を形式化しないことと、その理由を表す。1 行に一つの ID を書き、ID の後に理由を書く（`; trace:unformalized RFC8725-UTF8-APPLICATION モデルはデコード後の値を扱い、UTF-8 のバイト列を扱わない`）。その条項や GOJ を形式化するとしたら置く層のモデルのファイルに置く

条項と GOJ は、`trace:formalizes` で形式化するか、`trace:unformalized` で形式化しない理由を書きます。
どちらもない定義を、形式化を忘れたものとして機械的に見つけられるようにするためです。
形式化しない理由を条項や GOJ の定義に書かないのは、`docs/external/` と `docs/design/` が `formal/` に依存しないようにするためです。
Evidence が Unconfirmed の条項は、モデルが制約しないことで表すため、どちらも書きません。

関係（[§6](#6-relations-between-goj-and-clauses)）は、次の検査で確かめます。

- Relation が Satisfies と Accepts の行ごとに、GOJ と条項の両方の ID を挙げた `trace:checks` を持つ検査を置く
- Relation が Possibly Rejects の行ごとに、互換性の反例を期待し、GOJ と条項の両方の ID を挙げた `trace:checks` を持つ検査を置く
- 互換性の検査の期待する判定を変えるのは、Possibly Rejects の行を加えるか除く変更と同じ変更に限る。判定だけを変えると、反例として現れる非互換と、GOJ に記録した既知の非互換が食い違うためである
