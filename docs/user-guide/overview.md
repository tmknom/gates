# GATES Overview

GATES の導入を検討している人に向けて、GATES で得られるもの、仕組み、導入の前に知っておく制約、必要な準備を示します。
ユーザーガイドの各文書で使う用語の説明も、本文書の §6 にまとめています。

## 1. GATES で得られるもの

GATES（GitHub App Token Exchange System）は、GitHub Actions のワークフローに、指定したリポジトリだけを読める短命なトークンを発行する仕組みです。

ワークフローで別の非公開リポジトリを読むには、ジョブごとの `GITHUB_TOKEN` とは別の資格情報が要ります。
よく使われるのは次の 2 つですが、どちらにも弱点があります。

- **パーソナルアクセストークンを Secrets に置く**：個人のアカウントに紐づき、その人が組織を離れると動かなくなる。権限が広く、期限も長くなりがちである
- **GitHub App Private Key を Secrets に置く**：鍵を手に入れた人は、GitHub App のすべての権限を使える。鍵を置くワークフローの数だけ漏えいの経路が増える

GATES を使うと、ワークフローにはどちらの資格情報も置かずに済みます。
ワークフローは読みたいリポジトリを GATES に伝え、そのリポジトリだけを読める 1 時間のトークンを受け取ります。
GitHub App Private Key は運用者の AWS アカウントの KMS から出ません。

## 2. 仕組み

ワークフローは GATES の Action のステップを一つ置くだけで、トークンを受け取れます。

```mermaid
sequenceDiagram
    participant W as ワークフロー（Action）
    participant A as 運用者の AWS（GATES の Lambda）
    participant K as AWS KMS
    participant G as GitHub
    W->>A: GitHub が発行した OIDC JWT と、読みたいリポジトリを渡す
    A->>A: 呼び出し元を確かめ、要求を認可する
    A->>K: GitHub App JWT への署名を依頼する
    K-->>A: 署名
    A->>G: トークンの発行を要求する
    G-->>A: Installation Access Token
    A-->>W: 要求と照合したトークン
    Note over W,G: ジョブの終了時に、Action がトークンの失効を要求する
```

ワークフローの身元は、GitHub が実行のたびに発行する OIDC JWT で確かめます。
GATES は呼び出し元を名前ではなく不変 Owner ID で照合するため、改名や手放された名前の取得によって、許可オーナーになりすますことはできません。
GATES の Lambda はインターネットに公開された呼び出し口を持たず、許可オーナーのワークフローだけが AWS の IAM Role を通して呼び出せます。

## 3. 発行されるトークン

- **対象**：要求で列挙したリポジトリだけ
- **権限**：リポジトリの内容を読む `contents:read` だけ。GitHub がリポジトリの基本情報を読む `metadata:read` を加えることがある
- **期限**：発行から 1 時間。GATES は延長しない
- **使える範囲**：Action を置いたジョブの後続のステップだけ。ほかのジョブには渡せない
- **失効**：ジョブの終了時に、Action が失効を試みる。失効に失敗しても、期限を過ぎれば使えなくなる
- **一部だけの発行はしない**：要求したリポジトリのうち一つでも許可されなければ、どのリポジトリのトークンも返さない

## 4. 導入の前に知っておく制約

### 4.1 読める範囲はインストール対象で決まる

導入の判断で最も重い制約です。
GATES は、呼び出し元を許可オーナー単位でしか区別しません。
許可オーナーのワークフローであれば、どのワークフローも、インストール対象のどのリポジトリでも読めます。

**機密性の高いリポジトリをインストール対象に含めると、許可オーナーのすべてのワークフローから読めるようになります。**
呼び出し元ごとに読める範囲を絞る認可は将来の候補ですが、時期は決まっていません。
この制約のままで使えるかを基準に、導入を判断してください。
利用者の側で守ることは [Security Considerations](security-considerations.md) にあります。

### 4.2 そのほかの制約

- **許可オーナーは一つ**：一つの GATES がトークンを発行するのは、一つのオーナーのワークフローと、そのオーナーのリポジトリに対してだけである。複数のオーナーを扱う予定はない
- **権限は `contents:read` だけ**：書き込みや、Issue と Pull Request の操作には使えない
- **GitHub.com の GitHub Actions だけ**：GitHub Enterprise Server や、GitHub Actions 以外の実行環境には対応しない
- **リポジトリの OIDC JWT は不変の ID を含む形式に限る**：新しいリポジトリは既定でこの形式になる。古いリポジトリは、リポジトリか Organization の設定で切り替える
- **Runner は Linux で Docker を使えるもの**：GitHub がホストする `ubuntu-latest` はこれを満たす
- **Organization を許可オーナーにした構成は未検証**：GATES の検証環境で動作を確かめたのは、ユーザーアカウントを許可オーナーにした構成だけである

## 5. 役割と責任

GATES は、次の 3 つの役割がそれぞれの責任を果たすことを前提に動きます。
ユーザーアカウントを許可オーナーにする構成では、同じ人が 3 つを兼ねることが多くなります。

| 役割 | 担うこと |
|---|---|
| **GATES 運用者**（以下、運用者） | GATES を AWS にデプロイして運用する。GitHub App Private Key を KMS にインポートし、更新する。接続情報を GitHub に登録し、ワークフロー利用者に Action の参照先を伝える |
| **ワークフロー利用者** | GATES からトークンを受け取るワークフローを書く。トークンを受け取るジョブで、信頼できないコードを動かさない |
| **GitHub App とオーナーの管理者** | GitHub App のインストール対象を管理し、許可オーナーのどのワークフローから読まれてもよいリポジトリだけを含める。リポジトリの OIDC JWT の形式を設定する |

## 6. 用語

ユーザーガイドの各文書は、次の用語を説明なしで使います。

- **許可オーナー**：GATES がトークンを発行してよいと運用者が設定した GitHub のユーザーアカウントか Organization
- **不変 Owner ID**：GitHub がアカウントに割り当てる数値の ID。名前を変えても変わらない
- **インストール対象**：GitHub App を許可オーナーにインストールするときに選ぶリポジトリ。GATES で読める範囲になる
- **Installation**：GitHub App をオーナーにインストールした単位
- **トークン**：GATES が受け取って渡す、GitHub App の Installation Access Token
- **Action**：ワークフローがトークンを受け取るために置く、GATES の Docker Action
- **OIDC JWT**：ワークフローがどのオーナーのどのリポジトリで動いているかを、GitHub が署名して示すトークン
- **Audience**：OIDC JWT の宛先を示す値。GATES は、運用者が設定した値の OIDC JWT だけを受け付ける
- **接続情報**：Action が GATES を呼び出すのに使う値（AWS アカウント ID、IAM Role 名、関数名、リージョン、Audience）。運用者が GitHub の Secrets と Variables に登録する
- **監査記録**：GATES が要求ごとに、呼び出し元、要求、結果を、トークンを含めずに CloudWatch Logs に残す 1 行の JSON。応答と注釈に出る `request_id` で探せる
- **鍵素材**：KMS Key にインポートする GitHub App Private Key の中身。インポートした後は、別の鍵素材に入れ替えられない
- **使用中の鍵**：GATES の Lambda が署名に使う KMS Key
- **鍵の更新**：新しい KMS Key に新しい GitHub App Private Key をインポートし、使用中の鍵を切り替える作業
- **配布物**：GitHub Releases に置く、GATES の Lambda の関数のコードの zip
- **Attestation**：配布物をどのリポジトリのどのコミットから、どのワークフローでビルドしたかを示す、署名付きの記録

## 7. 運用者が用意するもの

- **AWS アカウント**：Lambda の関数、KMS Key、IAM Role、CloudWatch Logs のロググループを作る。作業には、セッショントークンを含む一時認証情報を使う
- **IAM OIDC Provider**：AWS が GitHub Actions の OIDC JWT を信頼するための、アカウントに一つだけ置く設定。なければ作る
- **GitHub App**：`Contents` を `Read-only` にして、許可オーナーにインストールする
- **手元のツール**：Git、`make`、Docker（Docker Compose を含む）、Go、`zip`、`shasum`、AWS CLI、`jq`、`xxd`、OpenSSL 3 以降、GitHub CLI

運用者の AWS アカウントでは、次のリソースに費用が発生します。
金額はリージョンと時期で変わるため、各サービスの料金ページで確かめてください。

| リソース | 課金の単位 |
|---|---|
| [AWS Lambda](https://aws.amazon.com/lambda/pricing/) | 呼び出しの回数と実行時間。トークンの要求 1 回につき 1 回呼び出す |
| [AWS KMS](https://aws.amazon.com/kms/pricing/) | KMS Key ごとの月額と、署名の回数。鍵の更新のあいだは KMS Key が 2 つになる |
| [Amazon CloudWatch Logs](https://aws.amazon.com/cloudwatch/pricing/) | ログの取り込み量と保存量。保持期間の既定は 7 日 |

## 8. さらに知りたい人へ

- [Constitution](../CONSTITUTION.md)：GATES の存在理由と、設計上の判断原則
- [Scope](../spec/scope.md)：現在の範囲、将来の候補、対象外とする範囲
- [System Architecture](../design/architecture.md)：コンポーネントと処理の流れ
- [AWS Infrastructure Design](../design/aws-infrastructure.md)：作成する AWS リソースの構成
