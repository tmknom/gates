# Z3 Models

このディレクトリには、GATES が受け取る GitHub OIDC JWT の、デコード済みの Claims Set に対する受理条件を SMT-LIB 2 で記述しています。
ソルバーは [Z3](https://github.com/Z3Prover/z3) です。
受理条件の正本は [GATES OIDC JWT Claims](../../docs/design/gates-oidc-jwt-claims.md) で、規範性の根拠は [RFC 7519 JSON Web Token](../../docs/external/standards/rfc7519.md)、[RFC 8725 JWT Best Current Practices](../../docs/external/standards/rfc8725.md)、[OpenID Connect Core 1.0](../../docs/external/standards/oidc-core.md)、[GitHub Actions OIDC Constraints](../../docs/external/github/oidc.md) と一次資料です。

検査は、確かめる対象で次の 2 つに分かれます。
実装が仕様どおりかは、このモデルではなく Go のテストが確かめます。

- **仕様の検証**：任意の入力について受理条件の性質を示す。受理条件が標準の要求を満たすこと、受理された JWT が 1 つの受信者にしか使えないこと、受理される時刻が途切れず期限を持つこと、受理の判定が参照する Claim だけで決まることなど
- **モデルの写しの確認**：GATES OIDC JWT Claims の Verification Cases の例と実測した JWT を受理述語に代入し、モデルが GATES OIDC JWT Claims を写していることを確かめる。写し間違えたモデルでは、仕様の検証も誤ったモデルの上で成り立つだけになるため

## 構成

仕様（`model/`）と、仕様に対する問い（`checks/`）を分けています。

| ファイル | 内容 |
|---|---|
| `model/values.smt2` | 型付きの JsonValue、MaybeClaim、Claims Set、ID の表記、Claim 名の分類、別名の述語 |
| `model/standards/jwt.smt2` | RFC 7519 と RFC 8725 の時刻と Audience の条項。GATES の値（Issuer、Audience、猶予の秒数）を含まず、MAY と OPTIONAL の条項は許す選択の述語、標準が値を決めない猶予は引数で表す |
| `model/standard.smt2` | 選択した RFC の受信者側の `iss` と `sub` の条件と、OIDC Core の ID Token の `sub` の ASCII と 255 文字の条件 |
| `model/github.smt2` | GitHub 公式の固定の Issuer |
| `model/gates-current.smt2` | 現行の GATES の受理条件 |
| `model/gates-proposals.smt2` | 別名を無視する候補と、意図的に誤らせた述語 |
| `checks/fixtures.smt2` | 受理される Claims Set の例 `sample` と、その 1 つの Claim を差し替えた受理の判定 `accepts-with` |
| `checks/consistency/` | 仕様の検証。標準の条項どうし、GATES の受理条件どうしが矛盾せず、満たす例が存在すること |
| `checks/safety/` | 仕様の検証。GATES の受理条件が、関係 Satisfies を書いた標準の条項を満たすこと |
| `checks/properties/` | 仕様の検証。任意の入力について成り立つ性質 |
| `checks/candidates/` | 仕様の検証。現行の受理条件と候補の比較 |
| `checks/mutations/` | 全称の性質が、誤らせた述語の誤りを反例として検出できること |
| `checks/examples/` | モデルの写しの確認。GATES OIDC JWT Claims の Verification Cases の行ごとの例と、実測した JWT 6 件 |
| `verify.sh` | 検査ごとにモデルと検査を連結して Z3 で解き、期待する判定と比べる |

各検査のファイルは、1 行目に期待する判定（`; expect: sat` か `; expect: unsat`）を、続くコメントに確かめる内容を書いています。
条項と GOJ を形式化した述語と、それらを確かめる検査は、[Traceability Policy](../../docs/contributing/traceability.md) の `trace:formalizes` と `trace:checks` の注釈で ID を挙げます。
`checks/examples/` のように 1 つのファイルに `check-sat` が複数あるときは、1 行目に判定を空白で区切って同じ順に並べ（`; expect: unsat sat`）、各例は `push` と `pop` で囲みます。

SMT-LIB にはファイルを読み込む仕組みがないため、`verify.sh` が `model/` を表の上から順に連結し、最後に `checks/fixtures.smt2` を連結します。
後のファイルは前のファイルの定義を使うため、ファイルを追加するときは `verify.sh` の連結の順序に、使う定義より後の位置で加えます。

## モデルの前提と限界

- 一つの Claims Set を `(Array String MaybeClaim)` として扱い、型、値、欠落を一元化する。`now` と期待する Audience は外部から与え、全称の性質ではどちらも任意の値とする
- 受理述語が参照する名前（`consulted-name`）は、10 個の Claim（必須の 9 個と、任意で検証する `nbf`）と 45 個の別名である。監査用の 8 個、既知で使わない名前、`repo_property_*`、未知の名前は受理の判定に使わない
- 別名は、`iss`、`aud`、`exp`、`iat`、`nbf` と Unicode の単純な Case Folding で一致する名前で、文字ごとの対応から組み立てる述語 `claim-alias` で表す
- JSON のバイト列、UTF-8、エスケープ後の重複した Claim 名、署名、JOSE と JWKS は対象外である。モデルは、妥当で重複のない JSON をデコードした後の値を表す
- 監査用の 8 個の Claim からワークフロー起動属性を取り出す規則（空文字列、文字列でない値、欠落を記録なしにする）は受理条件ではないため対象外で、Go のテストが確かめる。モデルが示すのは、監査用の名前が受理の判定に影響しないことだけである
- `sub` 内の ID と個別の Claim の一致と、ID の表記が正の int64 を表すことは、全称の性質としては示していない。Z3 が判定できない（`unknown`）ためで、どちらも `checks/examples/` の例で写しを確かめる
- 実測の 6 件は Audience を指定せずに取得したもので、GATES 用の Audience を指定した JWT の実測ではない。観測は GitHub が常にその形で発行することを保証しない

## 判定の読み方

- **一貫性の検査が SAT**：述語を同時に満たす例が存在し、書き起こした条項や受理条件が互いに矛盾しない
- **全称の性質と安全性の検査が UNSAT**：モデル化した前提の中で、任意の入力について性質が成り立つ。GATES の受理条件の一貫性の検査（受理される例の存在）と組にして評価する。安全性の検査は、標準が値を決めない猶予を、GATES が使う値（`exp` は 0 秒、`nbf` は 300 秒）に置いて確かめる
- **例が期待どおり**：モデルが、その例について GATES OIDC JWT Claims と同じ判定を返す。例のない入力について写しが正しいことまでは示さない
- **候補との差分探索が SAT**：現行の判定と候補の判定が異なる例が存在する
- **変異が SAT**：誤らせた述語のもとで全称の性質に反例があり、性質がその誤りを検出できる
- **合同性**：`PCONGRUENCE` は、参照する名前の値がすべて等しい 2 つの Claims Set の判定が等しいことを示す。参照しない名前（監査用の名前を含む）の値は、判定に影響しない

## 規則と出典

時刻と Audience の条項と GOJ は、述語と検査の `trace:` の注釈が ID で直接参照するため、この表に載せません。
表の規則 ID は、それ以外の規則についての、このモデルの中の ID です。
出典の `GOJ-` で始まる ID は、GATES OIDC JWT Claims の規則の ID です。

| 規則 ID | 述語 | 対象と適用主体 | 出典 | 状態 |
|---|---|---|---|---|
| V-JSON-VALUE | `JsonValue` / `MaybeClaim` | デコード後の Claims Set の形 | GOJ-CLAIMS-JSON | モデル上の前提で、デコード処理は証明しない |
| RFC-ISS/SUB-TYPE | `standard-selected` | `iss`、`sub` | RFC 7519 §4.1.1, §4.1.2 | 選択した型と一致の条件だけ |
| OIDC-SUB-255 | `oidc-sub-conditional` | OIDC Core の ID Token | OIDC Core §2、GOJ-SUB-LENGTH | ID Token の標準の要求。GATES の受理条件がこれを満たすことを `PSUBOIDC` で確かめる |
| GH-ISS | `github-issuer` | GitHub の発行者 | `docs/external/github/oidc.md` | 公式の仕様（固定の Issuer） |
| GH-SUB-IMMUTABLE | `gates-sub` の採用形式 | GitHub の発行条件と GATES の受理条件は別 | `docs/external/github/oidc.md`、GOJ-SUB-FORMAT | GitHub の発行形式は条件付き。GATES が不変の形式を要求する |
| G-ISS | `github-issuer` | GATES の Verifier、`iss` | GOJ-ISS | 採用済み。固定の文字列と完全に一致する |
| G-SUB | `gates-sub` | GATES の Verifier、`sub` の構文 | GOJ-SUB-FORMAT, GOJ-ID-MATCH | 採用済みの構文と ID の一致の条件だけ |
| G-SUB-ASCII | `gates-sub` | GATES の Verifier、`sub` の文字種と長さ | GOJ-SUB-LENGTH | 採用済み。ASCII だけで 255 文字以内 |
| G-ID-DECIMAL | `positive-int64-text` | 不変の ID と Run ID の文字列 | GOJ-ID-FORMAT | 採用済み。正の canonical な int64 の文字列 |
| G-ID-MATCH | `gates-sub` | `sub` 内の 2 つの ID と個別の Claim | GOJ-ID-MATCH | 採用済み。文字列の一致 |
| G-RUN | `gates-common-except-exp` | `run_id`、`run_attempt` | GOJ-RUN-ID, GOJ-RUN-ATTEMPT | 採用済み。どちらも必須の canonical な int64 |
| G-UNKNOWN | `consulted-name` | 既知で使わない Claim、監査用の Claim、未知の Claim | 受理の分類と未使用・任意・監査の各 Claim の規則, GOJ-TRIGGER-ATTRIBUTES, GOJ-DYNAMIC-CLAIM | 採用済み。妥当な JSON なら受理に影響しない（別名を除く）ことを `PCONGRUENCE` と `PAUDITINV` で確かめる |
| G-CASE-ALIAS | `claim-alias` / `aliases-absent` | `iss`、`aud`、`exp`、`iat`、`nbf` の 45 個の別名 | GOJ-CLAIM-NAME-ALIAS | 採用済み。Unicode の単純な Case Folding に限る。どの別名を加えても受理されないことを `PALIAS` で確かめる |
| C-EXACT-ALIAS | `gates-exact-name-candidate` | 厳密な名前の比較だけ | なし | 候補で未採用 |
| OBS-01–06 | `checks/examples/observed.smt2` | 実測した JWT の 10 個の Claim | [GitHub OIDC JWT](../../docs/research/github/oidc-jwt.md) | 観測だけ。発行の保証や GATES 用の Audience の実測ではない |

一次資料は [RFC 7519](https://www.rfc-editor.org/rfc/rfc7519.html)、[OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)、[GitHub の OIDC のリファレンス](https://docs.github.com/en/actions/reference/security/oidc)、[Unicode の CaseFolding.txt](https://www.unicode.org/Public/UCD/latest/ucd/CaseFolding.txt) です。

## 実行方法

Docker が必要です。

```sh
make z3
```

すべての検査が期待する判定と一致すると、`PASS: <一致した件数>/<一致した件数> checks match expected SAT/UNSAT results.` を出力します。

モデルの中で、`check-sat` ごとにソルバーに 8 秒のタイムアウトを指定しています。
`verify.sh` は検査ごとに独立した Z3 のプロセスを起動し、このタイムアウトの (`check-sat` の数 + 1) 倍をプロセスの上限にします。
そのため、遅い `check-sat` はその位置の判定が `unknown` になり、同じ検査のほかの判定は残ります。
判定は `sat`、`unsat`、`unknown`、`timeout`、`error` を区別し、期待と異なる検査、1 行目に期待する判定がない検査、期待する判定の数が `check-sat` の数と異なる検査があると `FAIL` の行を出して終了コード 1 で終わります。

## 検査の追加

1. `checks/` の該当するディレクトリに、1 行目に期待する判定を、2 行目に確かめる条項と GOJ を挙げた `; trace:checks` を、続くコメントに内容を書いた検査を追加する
2. `make z3` を実行する
3. リポジトリのルートで `go run ./formal/trace` を実行し、注釈の ID が定義されていること、関係 Satisfies の行ごとに両方の ID を挙げた `trace:checks` があることを確かめる。欠落を見つけると、ファイルと行を出力して終了コード 1 で終わる

GATES OIDC JWT Claims の受理条件を変えたら、`checks/examples/` の該当する例も同じ変更で直します。
全称の性質は、述語の定義を言い直すのではなく、定義と独立に立てられる性質（標準の要求、規則どうしの関係、判定が時刻や Audience によってどう変わるか）にします。
その性質が検出する誤りを `model/gates-proposals.smt2` の述語として書き、`checks/mutations/` に反例が見つかることを示す検査を足すと、性質の強さを確かめられます。
変異の検査で Z3 が反例を作れず `unknown` になるときは、`sub` などの値か反例そのものを固定して手掛かりを与えます。
