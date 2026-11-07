# Z3 Models

このディレクトリには、GATES が受け取る GitHub OIDC JWT の、デコード済みの Claims Set に対する受理条件を SMT-LIB 2 で記述しています。
ソルバーは [Z3](https://github.com/Z3Prover/z3) です。
受理条件の正本は [JWT Design](../../docs/design/jwt.md) で、規範性の根拠は [JWT Constraints](../../docs/spec/jwt-constraints.md)、[GitHub Constraints](../../docs/spec/github-constraints.md) と一次資料です。

検査は、確かめる対象で次の 2 つに分かれます。
実装が仕様どおりかは、このモデルではなく Go のテストが確かめます。

- **仕様の検証**：任意の入力について受理条件の性質を示す。受理条件が標準の要求を満たすこと、受理された JWT が 1 つの受信者にしか使えないこと、受理される時刻が途切れず期限を持つこと、受理の判定が参照する Claim だけで決まることなど
- **モデルの写しの確認**：JWT Design の代表テストの例と実測した JWT を受理述語に代入し、モデルが JWT Design を写していることを確かめる。写し間違えたモデルでは、仕様の検証も誤ったモデルの上で成り立つだけになるため

## 構成

仕様（`model/`）と、仕様に対する問い（`checks/`）を分けています。

| ファイル | 内容 |
|---|---|
| `model/values.smt2` | 型付きの JsonValue、MaybeClaim、Claims Set、ID の表記、Claim 名の分類、別名の述語 |
| `model/standard.smt2` | 選択した RFC の受信者側の条件と、OIDC Core の ID Token の `sub` の ASCII と 255 文字の条件 |
| `model/github.smt2` | GitHub 公式の固定の Issuer |
| `model/gates-current.smt2` | 現行の GATES の受理条件 |
| `model/gates-proposals.smt2` | 別名を無視する候補と、意図的に誤らせた述語 |
| `checks/fixtures.smt2` | 受理される Claims Set の例 `sample` と、その 1 つの Claim を差し替えた受理の判定 `accepts-with` |
| `checks/properties/` | 仕様の検証。受理される例の存在（`P01`）と、任意の入力について成り立つ性質 |
| `checks/candidates/` | 仕様の検証。現行の受理条件と候補の比較 |
| `checks/mutations/` | 全称の性質が、誤らせた述語の誤りを反例として検出できること |
| `checks/examples/` | モデルの写しの確認。JWT Design の代表テストの行ごとの例と、実測した JWT 6 件 |
| `verify.sh` | 検査ごとにモデルと検査を連結して Z3 で解き、期待する判定と比べる |

各検査のファイルは、1 行目に期待する判定（`; expect: sat` か `; expect: unsat`）を、続くコメントに検査の ID と確かめる内容を書いています。
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

- **全称の性質が UNSAT**：モデル化した前提の中で、任意の入力について性質が成り立つ。受理される例が存在すること（`P01`）と組にして評価する
- **例が期待どおり**：モデルが、その例について JWT Design と同じ判定を返す。例のない入力について写しが正しいことまでは示さない
- **候補との差分探索が SAT**：現行の判定と候補の判定が異なる例が存在する
- **変異が SAT**：誤らせた述語のもとで全称の性質に反例があり、性質がその誤りを検出できる
- **合同性**：`PCONGRUENCE` は、参照する名前の値がすべて等しい 2 つの Claims Set の判定が等しいことを示す。参照しない名前（監査用の名前を含む）の値は、判定に影響しない

## 規則と出典

| 規則 ID | 述語 | 対象と適用主体 | 出典 | 状態 |
|---|---|---|---|---|
| V-JSON-VALUE | `JsonValue` / `MaybeClaim` | デコード後の Claims Set の形 | `jwt.md` §2.38.4 | モデル上の前提で、デコード処理は証明しない |
| RFC-EXP-TIME | `standard-time-selected` | JWT の受信者、`exp` | RFC 7519 §4.1.4 | MUST。猶予は MAY で、選択値は 0 秒 |
| RFC-NBF-TIME | `standard-time-selected` | JWT の受信者、`nbf` | RFC 7519 §4.1.5 | MUST。猶予は MAY で、比較のモデルは 300 秒 |
| RFC-IAT-TYPE | `standard-time-selected` | JWT の `iat` | RFC 7519 §4.1.6 | NumericDate。未来 300 秒の規則は RFC の MUST ではない |
| RFC-AUD-RECEIVER | `standard-aud-selected` | JWT の受信者、Audience の包含 | RFC 7519 §4.1.3 | 受信者が含まれることは MUST。単一の Audience は追加の制約 |
| RFC-ISS/SUB-TYPE | `standard-selected` | `iss`、`sub` | RFC 7519 §4.1.1, §4.1.2 | 選択した型と一致の条件だけ |
| OIDC-SUB-255 | `oidc-sub-conditional` | OIDC Core の ID Token | OIDC Core §2、`jwt.md` §2.2 | ID Token の標準の要求。GATES の受理条件がこれを満たすことを `PSUBOIDC` で確かめる |
| GH-ISS | `github-issuer` | GitHub の発行者 | `github-constraints.md` §3.1 | 公式の仕様（固定の Issuer） |
| GH-SUB-IMMUTABLE | `gates-sub` の採用形式 | GitHub の発行条件と GATES の受理条件は別 | `github-constraints.md` §3.2、`jwt.md` §2.2 | GitHub の発行形式は条件付き。GATES が不変の形式を要求する |
| G-EXP | `gates-exp` | GATES の Verifier、`exp` | `jwt.md` §2.4, §2.38.3 | 採用済み。整数の字句、0..253402300799、`now < exp`。受理される時刻に上限があることを `PTIMEBOUNDED` で確かめる |
| G-IAT | `gates-iat` | GATES の Verifier、`iat` | `jwt.md` §2.5, §2.38.3 | 採用済み。未来の上限は 300 秒。時刻の 3 つの規則を合わせて、受理される時刻が区間をなすことを `PTIMEINTERVAL` で確かめる |
| G-NBF | `gates-nbf` | GATES の Verifier、`nbf` | `jwt.md` §2.8, §2.38.3 | 採用済み。欠落は許し、型の違いは拒否する |
| G-ISS | `github-issuer` | GATES の Verifier、`iss` | `jwt.md` §2.6 | 採用済み。固定の文字列と完全に一致する |
| G-AUD | `gates-aud` | GATES の Verifier、`aud` | `jwt.md` §2.3 | 採用済み。単一の文字列か、要素が一つの配列だけ。受理された Claims Set が 1 つの Audience でしか受理されないことを `PAUDUNIQUE` で確かめる |
| G-SUB | `gates-sub` | GATES の Verifier、`sub` の構文 | `jwt.md` §2.2(1–5), §2.38.2 | 採用済みの構文と ID の一致の条件だけ |
| G-SUB-ASCII | `gates-sub` | GATES の Verifier、`sub` の文字種と長さ | `jwt.md` §2.2(6) | 採用済み。ASCII だけで 255 文字以内 |
| G-ID-DECIMAL | `positive-int64-text` | 不変の ID と Run ID の文字列 | `jwt.md` §2.38.1 | 採用済み。正の canonical な int64 の文字列 |
| G-ID-MATCH | `gates-sub` | `sub` 内の 2 つの ID と個別の Claim | `jwt.md` §2.38.2 | 採用済み。文字列の一致 |
| G-RUN | `gates-common-except-exp` | `run_id`、`run_attempt` | `jwt.md` §2.17, §2.19 | 採用済み。どちらも必須の canonical な int64 |
| G-UNKNOWN | `consulted-name` | 既知で使わない Claim、監査用の Claim、未知の Claim | `jwt.md` §1.3, §2.38.4–5 | 採用済み。妥当な JSON なら受理に影響しない（別名を除く）ことを `PCONGRUENCE` と `PAUDITINV` で確かめる |
| G-CASE-ALIAS | `claim-alias` / `aliases-absent` | `iss`、`aud`、`exp`、`iat`、`nbf` の 45 個の別名 | `jwt.md` §2.38.4 | 採用済み。Unicode の単純な Case Folding に限る。どの別名を加えても受理されないことを `PALIAS` で確かめる |
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

1. `checks/` の該当するディレクトリに、1 行目に期待する判定を、続くコメントに ID と内容を書いた検査を追加する
2. `make z3` を実行する

JWT Design の受理条件を変えたら、`checks/examples/` の該当する例も同じ変更で直します。
全称の性質は、述語の定義を言い直すのではなく、定義と独立に立てられる性質（標準の要求、規則どうしの関係、判定が時刻や Audience によってどう変わるか）にします。
その性質が検出する誤りを `model/gates-proposals.smt2` の述語として書き、`checks/mutations/` に反例が見つかることを示す検査を足すと、性質の強さを確かめられます。
変異の検査で Z3 が反例を作れず `unknown` になるときは、`sub` などの値か反例そのものを固定して手掛かりを与えます。
