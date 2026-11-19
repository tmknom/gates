# Z3 Models

このディレクトリには、GATES が受け取る GitHub OIDC JWT の、デコード済みの JOSE Header と Claims Set を SMT-LIB 2 で記述しています。
ソルバーは [Z3](https://github.com/Z3Prover/z3) です。
受理条件の正本は [GATES OIDC JWT Claims](../../docs/design/gates-oidc-jwt-claims.md) と [GATES OIDC JWT Signature](../../docs/design/gates-oidc-jwt-signature.md)、外部仕様の条項の正本は [External](../../docs/external/README.md) に挙げた標準仕様の文書と [GitHub Actions OIDC Constraints](../../docs/external/github/oidc.md) です。

モデルは、互いの記述を見ずに書く 3 つの層からなります。

- **標準の層（S）**：RFC と OpenID Connect の条項。GATES の値（GitHub の Issuer、GATES 用 Audience、猶予の秒数、受理するアルゴリズム）を含まない
- **GitHub の層（G）**：GitHub が発行する JWT についての `GH-OIDC-` の条項。Evidence で 2 段に分ける
- **GATES の層（A）**：GATES の受理条件（GOJ）。一つの述語が一つの GOJ の判定を表す

検査は、層どうしの関係を確かめます。
実装が受理条件どおりかは、このモデルではなく Go のテストが確かめます。

## 構成

層の定義（`model/`）と、層に対する問い（`checks/`）を分けています。

| ファイル | 内容 |
|---|---|
| `model/values.smt2` | 型付きの JsonValue、Member、JOSE Header と Claims Set、どの層の条項でもない共通の述語 |
| `model/standards/` | 標準の層。出典の資料ごとのファイル（`rfc7519.smt2`、`oidc-core.smt2` など）で、条項の ID の接頭辞と一対一に対応する。書き方の規則は `rfc7159.smt2` の冒頭にある |
| `model/github/documented.smt2` | GitHub の層の 1 段目。Evidence が Documented の条項と、公式の JWT の例 |
| `model/github/observed.smt2` | GitHub の層の 2 段目。Evidence が Published、Observed、Inferred の条項 |
| `model/gates/claims.smt2` | GATES の層の Claims Set の受理条件 |
| `model/gates/header.smt2` | GATES の層の JOSE Header の受理条件 |
| `model/proposals.smt2` | 現行の設計を変えない候補と、変異の検査のために意図的に誤らせた 3 つの層の述語 |
| `checks/fixtures.smt2` | 受理される例（`sample`、`sample-header`）、互換性の検査が使う GitHub の発行（`documented-issuance`、`observed-issuance`）と、それを絞る前提 |
| `checks/consistency/` | 一貫性の検査 |
| `checks/safety/` | 安全性の検査 |
| `checks/compat/` | 互換性の検査 |
| `checks/examples/` | Verification Cases の例、公式の JWT の例、実測した JWT 6 件 |
| `checks/properties/` | GATES の層の中で成り立つ性質（受信者の一意性、受理される時刻の連続性など） |
| `checks/candidates/` | 現行の受理条件と候補の比較 |
| `checks/mutations/` | 検査が、誤らせた述語の誤りを検出できること |
| `verify.sh` | 検査ごとにモデルと検査を連結して Z3 で解き、期待する判定と比べる |

SMT-LIB にはファイルを読み込む仕組みがないため、`verify.sh` が `model/` を `values.smt2`、標準の層、GitHub の層、GATES の層、`proposals.smt2` の順に連結し、最後に `checks/fixtures.smt2` を連結します。
3 つの層は `values.smt2` だけを共有し、互いの定義を使いません。
層の中では後のファイルが前のファイルの定義を使うため、ファイルを加えるときは `verify.sh` の連結の順序に、使う定義より後の位置で加えます。

## 層の書き方

述語と検査は、[Traceability Policy](../../docs/contributing/traceability.md) の注釈で文書の ID を参照します。
形式化した述語の直前の行に `trace:formalizes`、検査の 2 行目に `trace:checks`、形式化しない条項と GOJ には、形式化するとしたら置く層のファイルに `trace:unformalized` と理由を書きます。
JSON のバイト列、UTF-8、直列化のように、デコード後の値を扱うモデルが表せない条項は、`trace:unformalized` にします。

- **標準の層**：MAY と OPTIONAL の条項は許す選択の述語にし、標準が値を決めない猶予やアルゴリズムの集合は引数にする。条項の Condition は Bool の引数にする。GATES の値は検査の側で与える
- **GitHub の層**：Claim の値の条項は、Claim がある場合の値として書く。Claim が含まれる条件を定める Documented の条項がないため（GH-OIDC-CLAIM-PRESENCE）。Evidence が Unconfirmed の条項は書かない。制約しないことで、互換性の検査がその条項の許す発行を見つけられるようにするためである
- **GitHub の層の 2 段**：Published と Observed は、記録した時点と条件での値で、GitHub の保証ではないため、どちらも 2 段目に置く。Inferred の条項は、導いた元の条項の段に置く
- **GATES の層**：使わない Claim と Header Parameter の GOJ は、使わない名前の集合として書き、受理がその名前を読まないことを検査で確かめる

## 検査の種類

各検査のファイルは、1 行目に期待する判定（`; expect: sat` か `; expect: unsat`）を、2 行目に `trace:checks` を、続くコメントに確かめる内容を書いています。
1 つのファイルに `check-sat` が複数あるときは、1 行目に判定を空白で区切って同じ順に並べ、各 `check-sat` を `push` と `pop` で囲みます。

- **一貫性（`consistency/`）**：各層の条項が矛盾せず、満たす例が存在する。期待は SAT。書き起こした条項が互いに矛盾すれば、どれかの読み違いを疑える。GATES の層の例は、全称の検査が空虚に成り立っていないことも示す
- **安全性（`safety/`）**：GOJ が受理するすべての入力が、関係 Satisfies を書いた標準の条項を満たす。期待は UNSAT。標準が値を決めない猶予やアルゴリズムの集合は、GATES が使う値に置いて確かめる
- **互換性（`compat/`）**：GitHub の層の発行を GOJ が受理するか。3 つの形がある
    - **Accepts の検査**：関係 Accepts の行の条項の段の G に、Possibly Rejects の行が表す発行を除く前提を加え、GOJ が受理することを確かめる。期待は UNSAT。2 段目の検査は、1 段目の条項も含む。除く行は、その GOJ の行と、その GOJ の判定が含む GOJ（ID の Claim の GOJ が含む GOJ-ID-FORMAT など）や、その GOJ の判定が値の型を前提とする GOJ（GOJ-TIME-NBF が前提とする GOJ-NBF など）の行である。条項が値を制約しない GOJ（GOJ-RUN-ID など）では、前提が受理の条件そのものになり、検査は受理が Possibly Rejects の行の外の発行に限られることだけを示す
    - **依存の検査**：1 段目の G だけでは GOJ が受理しない発行が残ることを示す。期待は SAT で、`trace:checks` には GOJ だけを挙げる。GATES の受理が 2 段目の条項に依存していることを表す
    - **Possibly Rejects の検査**：Possibly Rejects の行ごとに、1 段目の G が許し、GOJ が拒否する発行があることを確かめる。期待は SAT。Possibly Rejects の行と一対一に対応する
- **例（`examples/`）**：Verification Cases の例を GATES の層に、公式の JWT の例を GitHub の層の例の条項として、実測した JWT を 3 つの層すべてに代入する。実測値が標準の層を満たさなければ、読み違いか GitHub の仕様違反を示す
- **性質（`properties/`）**：GATES の層の中で、定義と独立に立てられる性質。期待は UNSAT
- **候補（`candidates/`）**：現行の受理条件と候補の差分
- **変異（`mutations/`）**：`model/proposals.smt2` の誤らせた述語のもとで、検査の判定が変わることを示す。GATES の層の変異では、全称の検査に反例が見つかる（SAT）。標準の層と GitHub の層の変異では、一貫性の検査の例か、互換性の検査の反例がなくなる（UNSAT）

互換性の検査の期待する判定は、関係 Possibly Rejects の行を加えるか除く変更と同じ変更でだけ変えます。

## 判定の読み方

- **一貫性の検査が SAT**：述語を同時に満たす例が存在し、書き起こした条項や受理条件が互いに矛盾しない
- **安全性と性質の検査が UNSAT**：モデル化した前提の中で、任意の入力について性質が成り立つ。GATES の層の一貫性の検査（受理される例の存在）と組にして評価する
- **Accepts の検査が UNSAT**：その段の G が許し、Possibly Rejects の行が表す発行を除いた、すべての発行を GOJ が受理する。前提に置いた除外は、検査のコメントに条項の ID で書いている
- **依存の検査が SAT**：Documented の条項だけでは、GATES の受理が成り立たない。2 段目の条項（記録した値）が変われば、拒否が起こり得る
- **Possibly Rejects の検査が SAT**：GitHub の条項が許す発行を、GOJ が拒否する。GATES が受け入れた既知の非互換で、GitHub が実際にその発行を行うかは、多くの場合は確定していない
- **例が期待どおり**：モデルが、その例について文書と同じ判定を返す。例のない入力について写しが正しいことまでは示さない
- **変異の判定が期待どおり**：検査がその誤りを検出でき、誤った述語のもとでも同じ判定になるほど弱くはない

## モデルの前提と限界

- JOSE Header と Claims Set を `(Array String Member)` として扱い、型、値、欠落を一元化する。`now` と期待する Audience は外部から与え、全称の性質ではどちらも任意の値とする
- 受理述語が読む Claim の名前（`consulted-name`）は、10 個の Claim（必須の 9 個と、任意で検証する `nbf`）と 45 個の別名である。Header Parameter の名前（`header-consulted-name`）は、`alg`、`kid`、`cty`、`crit`、`b64`、`jwk`、`x5c`、`nonce` の 8 個である
- JSON のバイト列、UTF-8、エスケープ後の重複した名前、JWS Compact Serialization、署名、Discovery、JWK と JWK Set は対象外である。モデルは、妥当で重複のない JSON をデコードした後の値を表す。`jwk` が JWK として、`x5c` が X.509 証明書の配列として読めるかは、オブジェクトであること、空でない文字列の配列であることで近似する
- 監査用の 8 個の Claim からワークフロー起動属性を取り出す規則は受理条件ではないため対象外で、Go のテストが確かめる。モデルが示すのは、監査用の名前が受理の判定に影響しないことだけである
- `sub` の解析は、任意の名前と ID について全称の性質として示していない。Z3 が `str.indexof` の位置を決められず `unknown` を返すためである。互換性の検査は、名前と ID と Context を公式資料と実測の値に固定し、残りの発行を任意にする。ほかの値は `checks/examples/` の例と Go のテストが確かめる
- 互換性の検査が前提に置く時計（GitHub の時計が Lambda の時計より `L` 以上進まないこと、時計が `exp` の範囲にあること）は、GitHub の条項ではなく、検査の側の前提である
- GitHub の層の 2 段目の条項は、どれも個人アカウントの private リポジトリで記録した値である（GH-OIDC-ORGANIZATION-JWT）。実測の 6 件は Audience を指定せずに取得したもので、GATES 用 Audience を指定した JWT の値の記録ではない。互換性の検査の 2 段目は、Audience を指定した JWT も同じ値を持つことを、検査の側の前提として置く。Audience を指定した JWT で記録したのは、`aud` と一部の Claim の型と Header の名前だけである（GH-OIDC-AUD-CUSTOM-STRING、GH-OIDC-CLAIM-TYPE-OBSERVED、GH-OIDC-HEADER-OBSERVED）
- 互換性の検査の前提（`documented-issuance`、`observed-issuance` と、それを絞る前提）を満たす発行があることは、GitHub の層の一貫性の検査が確かめる。前提が矛盾すれば、UNSAT を期待する検査は空虚に成り立つためである

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

1. 条項か GOJ を加えたら、その層のファイルに述語を書き、直前の行に `trace:formalizes` を置く。形式化しないなら `trace:unformalized` と理由を書く
2. 関係の表の行を加えたら、その種類に応じた検査を `checks/` の該当するディレクトリに加える。Satisfies の行は安全性の検査、Accepts の行は Accepts の検査、Possibly Rejects の行は Possibly Rejects の検査で、どれも GOJ と条項の両方の ID を `trace:checks` に挙げる。GOJ か条項が `trace:unformalized` の行は、Go のテストに両方の ID を挙げた `trace:checks` を置く
3. `make z3` を実行する
4. `make trace` を実行し、注釈と関係の表が [Traceability Policy](../../docs/contributing/traceability.md) の規則を満たすことを確かめる

受理条件を変えたら、`checks/examples/` の該当する例も同じ変更で直します。
全称の性質は、述語の定義を言い直すのではなく、定義と独立に立てられる性質（標準の要求、規則どうしの関係、判定が時刻や Audience によってどう変わるか）にします。
その性質が検出する誤りを `model/proposals.smt2` の述語として書き、`checks/mutations/` に判定が変わることを示す検査を足すと、性質の強さを確かめられます。
Z3 が反例を作れず `unknown` になるときは、`sub` などの値か反例そのものを固定して手掛かりを与えます。
