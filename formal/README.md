# Formal Models

このディレクトリには、GATES の仕様を形式手法で検証するモデルを、ツールごとのディレクトリに分けて置いています。

| ディレクトリ | ツール | 対象 | 実行 |
|---|---|---|---|
| [Alloy Models](alloy/README.md) | Alloy | 認可と発行処理の規則 | `make alloy` |
| [Z3 Models](z3/README.md) | Z3 | GitHub OIDC JWT の Claims Set の受理条件 | `make z3` |

仕様の正本は [Requirements](../docs/spec/requirements.md) と `docs/design/` の設計文書です。
モデルと仕様が食い違ったら、どちらが正しいかを検討し、仕様を変える場合は正本を先に直します。
モデルの概念の名前は [Domain Model](../docs/design/domain.md) の英語名に揃えます。
