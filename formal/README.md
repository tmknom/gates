# Formal Models

このディレクトリには、GATES の仕様を形式手法で検証するモデルを、ツールごとのディレクトリに分けて置いています。

| ディレクトリ | ツール | 対象 | 実行 |
|---|---|---|---|
| [Alloy Models](alloy/README.md) | Alloy | 認可と発行処理の規則と、GitHub OIDC JWT の署名の検証鍵（Discovery、JWK、JWK Set、`kid` による選択、署名）の受理条件と、標準仕様と GitHub の条項との関係 | `make alloy` |
| [Z3 Models](z3/README.md) | Z3 | GitHub OIDC JWT の JOSE Header と Claims Set の受理条件と、標準仕様と GitHub の条項との関係 | `make z3` |
| `trace/` | Go | `docs/` が定義する ID と、文書、モデル、テストの参照と `trace:` の注釈との対応 | `make trace` |

`make alloy`、`make z3`、`make trace` はリポジトリのルートからも、このディレクトリからも実行できます。
このディレクトリの `make all` は 3 つを続けて実行します。

追跡の検査は、[Traceability Policy](../docs/contributing/traceability.md) の規則を満たさない箇所を `ファイル:行: 内容` の形で出力し、一つでもあれば失敗します。
`docs/` の定義、関係の表、`formal/` の注釈、`internal/` のテストの注釈を変えたら、`make trace` で確かめます。
報告ごとの直し方は、Traceability Policy の Trace Check の節にあります。
条項と GOJ を形式化する述語と、確かめる検査とテストを探すときは、`make trace-map` が出力する表を使います。

仕様の正本は [Requirements](../docs/spec/requirements.md)、[Acceptance Criteria](../docs/spec/acceptance-criteria.md)、`docs/design/` の設計文書です。
モデルと仕様が食い違ったら、どちらが正しいかを検討し、仕様を変える場合は正本を先に直します。
モデルの概念の名前は [Domain Model](../docs/design/domain.md) の英語名に揃えます。
