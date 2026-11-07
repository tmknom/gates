# Z3 Models

このディレクトリには、GATES が受け取る GitHub OIDC JWT の、デコード済みの Claims Set に対する受理条件を SMT-LIB 2 で記述しています。
ソルバーは [Z3](https://github.com/Z3Prover/z3) です。
受理条件の正本は [JWT Design](../../docs/design/jwt.md) で、規範性の根拠は [JWT Constraints](../../docs/spec/jwt-constraints.md)、[GitHub Constraints](../../docs/spec/github-constraints.md) と一次資料です。

## 構成

仕様（`model/`）と、仕様に対する問い（`checks/`）を分けています。

| ファイル | 内容 |
|---|---|
| `model/values.smt2` | 型付きの JsonValue、MaybeClaim、Claims Set、ID の表記、Claim 名の分類 |
| `model/projection.smt2` | 受理述語が読む 46 個の名前を、一つの CoreView に投影する |
| `model/standard.smt2` | 選択した RFC の受信者側の条件と、OIDC Core の ID Token の `sub` の ASCII と 255 文字の条件 |
| `model/github.smt2` | GitHub 公式の固定の Issuer。観測値は検証事例として `checks/observations/` で別に扱う |
| `model/gates-current.smt2` | 現行の GATES の受理条件。Unicode の別名の規則は不完全である |
| `model/gates-proposals.smt2` | 別名を無視する候補と、意図的に誤らせた 3 つの述語 |
| `model/audit.smt2` | 任意の監査 Claim の取り出しと、空文字列の正規化。受理の判定とは独立している |
| `checks/fixtures.smt2` | 受理される Claims Set の例 |
| `checks/properties/` | 充足可能性、含意、投影の不変性、候補との比較 |
| `checks/boundaries/` | 時刻、ID、`sub`、`aud` の境界値 |
| `checks/mutations/` | 正しい仕様と誤った仕様の判定の差 |
| `checks/observations/` | 実測した GitHub の JWT 6 件から、10 個の Claim だけを投影した例 |
| `checks/expected.txt` | 検査の ID、ファイル、期待する判定の一覧 |
| `verify.sh` | 検査ごとにモデルと検査を連結して Z3 で解き、期待する判定と比べる |

各検査のファイルの先頭のコメントに、検査の ID と確かめる内容を書いています。

SMT-LIB にはファイルを読み込む仕組みがないため、`verify.sh` が `model/` と `checks/fixtures.smt2` を表の上から順に連結します。
後のファイルは前のファイルの定義を使うため、ファイルを追加するときは `verify.sh` の連結の順序に、使う定義より後の位置で加えます。

## モデルの前提と限界

- 一つの Claims Set を `(Array String MaybeClaim)` として扱い、型、値、欠落を一元化する。`now` と期待する Audience は外部から与える
- 受理述語が参照するのは 10 個の Claim（必須の 9 個と、任意で検証する `nbf`）である。任意の監査用の 8 個、既知で使わない 18 個、`repo_property_*`、未知の名前は、原則として受理の判定に使わない
- 別名の拒否は近似である。ASCII の大文字と小文字の組み合わせは全 35 通りを、Unicode は `iſſ` の 1 例だけを表す。Unicode 全域の Case Folding や比較方式は定義していないため、`gates-current-example-aliases` を現行の契約の完全な表現とは読めない
- JSON のバイト列、UTF-8、エスケープ後の重複した Claim 名、署名、JOSE と JWKS は対象外である。モデルは、妥当で重複のない JSON をデコードした後の値を表す
- 観測の 6 件の Audience は既定値で、GATES 用の Audience を指定して発行した JWT の実測ではない。観測は GitHub が常にその形で発行することを保証しない

## 判定の読み方

- **反例探索が UNSAT**：モデル化した前提の中で、指定した違反が生じない。受理される例が存在すること（`P01`）と組にして評価する
- **差分探索が SAT**：現行の判定と、候補や誤った仕様の判定が異なる例が存在する
- **不変性**：`PINVAR` で変更に対する投影の同値を、`PCONGRUENCE` で受理述語の合同性を確かめ、`core-view` の 46 個の値が等しければ受理の判定も等しいことを示す。46 個の名前が受理述語の参照を網羅することはモデルの構造で担保するが、現実の Unicode の折り畳みは網羅しない
- **充足可能性が SAT**：設計と両立するモデルが存在することだけを意味し、GitHub が発行できることは保証しない

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
| G-EXP | `gates-exp` | GATES の Verifier、`exp` | `jwt.md` §2.4, §2.38.3 | 採用済み。整数の字句、0..253402300799、`now < exp` |
| G-IAT | `gates-iat` | GATES の Verifier、`iat` | `jwt.md` §2.5, §2.38.3 | 採用済み。未来の上限は 300 秒 |
| G-NBF | `gates-nbf` | GATES の Verifier、`nbf` | `jwt.md` §2.8, §2.38.3 | 採用済み。欠落は許し、型の違いは拒否する |
| G-ISS | `github-issuer` | GATES の Verifier、`iss` | `jwt.md` §2.6 | 採用済み。固定の文字列と完全に一致する |
| G-AUD | `gates-aud` | GATES の Verifier、`aud` | `jwt.md` §2.3 | 採用済み。単一の文字列か、要素が一つの配列だけ |
| G-SUB | `gates-sub` | GATES の Verifier、`sub` の構文 | `jwt.md` §2.2(1–5), §2.38.2 | 採用済みの構文と ID の一致の条件だけ |
| G-SUB-ASCII | `gates-sub` | GATES の Verifier、`sub` の文字種と長さ | `jwt.md` §2.2(6) | 採用済み。ASCII だけで 255 文字以内 |
| G-ID-DECIMAL | `positive-int64-text` | 不変の ID と Run ID の文字列 | `jwt.md` §2.38.1 | 採用済み。正の canonical な int64 の文字列 |
| G-ID-MATCH | `gates-sub` | `sub` 内の 2 つの ID と個別の Claim | `jwt.md` §2.38.2 | 採用済み。文字列の一致 |
| G-RUN | `gates-common-except-exp` | `run_id`、`run_attempt` | `jwt.md` §2.17, §2.19 | 採用済み。どちらも必須の canonical な int64 |
| G-UNKNOWN | `core-view` との照合 | 既知で使わない Claim、監査用の Claim、未知の Claim | `jwt.md` §1.3, §2.38.4–5 | 採用済み。妥当な JSON なら受理に影響しない（別名の特例を除く） |
| G-CASE-ALIAS | `recognized-alias-example` / `alias-examples-absent` | `iss`、`aud`、`exp`、`iat`、`nbf` だけ | `jwt.md` §2.38.4, §7 | 既存の設計で要レビュー。ASCII の 35 通りと Unicode の 1 例の近似に限る |
| C-EXACT-ALIAS | `gates-exact-name-candidate` | 厳密な名前の比較だけ | なし | 候補で未採用 |
| AUDIT-EXTRACT | `audit-extract-string` | 監査用の 8 個の Claim | `jwt.md` §2.38.4 | 受理から独立した抽出。文字列なら読み取れる |
| AUDIT-NORMALIZE | `audit-attribute-present` | 監査記録に残す属性 | `domain.md` §3.3 | 空文字列を欠落として扱う。受理の判定に関係しない |
| OBS-01–06 | `checks/observations/fixture*.smt2` | 実測した JWT の 10 個の Claim の投影 | [GitHub OIDC JWT](../../docs/research/github/oidc-jwt.md) | 観測だけ。発行の保証や GATES 用の Audience の実測ではない |

一次資料は [RFC 7519](https://www.rfc-editor.org/rfc/rfc7519.html)、[OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)、[GitHub の OIDC のリファレンス](https://docs.github.com/en/actions/reference/security/oidc) です。

## 実行方法

Docker が必要です。

```sh
make z3
```

すべての検査が期待する判定と一致すると、次を出力します。

```text
PASS: 96/96 checks match expected SAT/UNSAT results.
```

`verify.sh` は検査ごとに独立した Z3 のプロセスを起動し、16 秒の上限を設けます。
モデルの中でも、ソルバーに 8 秒のタイムアウトを指定しています。
判定は `sat`、`unsat`、`unknown`、`timeout`、`error` を区別し、期待と異なる検査があると `FAIL` の行を出して終了コード 1 で終わります。

## 検査の追加

1. `checks/` の該当するディレクトリに、先頭のコメントで ID と内容を示した検査を追加する
2. `checks/expected.txt` に `ID|ファイル|期待する判定` の行を追加する
3. `make z3` を実行する

観測の検査は ID を `OBS<番号>OK` か `OBS<番号>AUD` にすると、`verify.sh` が同じ番号の `fixture<番号>.smt2` を連結します。
