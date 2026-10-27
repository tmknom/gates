# GitHub Actions

## 1. Responsibility

Composite Action は、GitHub Actions の利用者にトークン取得手段を提供します。AWS 用 GitHub OIDC JWT による一時認証情報の取得、GATES 用 OIDC JWT の取得、Lambda の同期 Invoke、応答の処理と同一ジョブ内の後続ステップへのトークン出力を担当します。発行許可の判定は Lambda 側が行います。

Composite Action は GATES 運用者が提供し、利用者向けの安定した境界とします。利用者に求めるのは、ワークフローでの `permissions: id-token: write` の宣言と、対象リポジトリおよび権限の指定だけです。

IAM Role と Lambda の AWS リソース設定は [AWS Infrastructure Design](aws-infrastructure.md) に記述します。Lambda Invoke の Payload、応答、エラーコードは [Lambda Design](lambda.md) を正本とします。

## 2. Composite Action Interface

### 2.1 Inputs and Outputs

| Input | 必須 | 意味 |
|---|---|---|
| `repositories` | はい | カンマ区切りの `owner/repo` リスト |
| `permissions` | はい | カンマ区切りの `permission:level`。値は `contents:read` |

引き受ける IAM Role ARN、Lambda 関数名、AWS リージョン、GATES 用 Audience は利用者の入力にせず、GATES 運用者が Composite Action の中に固定します。GATES は単一の Organization に対する単一のデプロイであり、利用者が呼び出し先を選ぶ場面はないためです。固定することで、利用者の作業を対象と権限の指定だけにとどめ、呼び出し先の差し替えを Composite Action の更新に一本化できます。`scope` も入力にせず、Lambda Payload に固定値 `repository` を設定します。

| Output | 意味 |
|---|---|
| `token` | 発行された Installation Access Token |
| `expires-at` | GitHub が定めた有効期限（RFC 3339） |
| `repositories` | Lambda 応答が示す発行対象のリポジトリ。入力と同じ形式のカンマ区切りの `owner/repo` |
| `permissions` | Lambda 応答が示す実効権限。入力と同じ形式のカンマ区切りの `permission:level`。要求にかかわらず GitHub が付与する `metadata:read` を含み得る |

対象と実効権限を Output に含めるのは、利用者が受け取ったトークンの範囲を確認できるようにするためです（[Requirements](../spec/requirements.md) FR-016）。Output は呼び出したステップの `steps.<id>.outputs` から同一ジョブ内の後続ステップが利用できます。Action はジョブ間のトークン転送を提供しません。内部ステップの `GITHUB_OUTPUT` に設定した値は `action.yml` の `outputs` へマッピングします。

### 2.2 List Syntax

```yaml
with:
  repositories: example-org/repo-a,example-org/repo-b
  permissions: contents:read
```

区切りは `,` とします。要素前後の空白は除去し、空要素・重複・無効な形式は拒否します。Permission の値が許可条件を満たすかは Composite Action では判定せず、Lambda の認可に委ねます。認証済み Caller とともに許可外の要求を監査記録に残すためです（[Security Design](security.md#31-authorization-rule)）。Composite Action 内では `repositories` を配列、`permissions` を JSON オブジェクトへ変換します。独自のエスケープ構文は設けません。

## 3. Processing Flow

1. `aws-actions/configure-aws-credentials` を用い、固定した IAM Role とリージョンで AWS STS の一時認証情報を取得する。
2. GitHub が OIDC JWT の取得用に提供する環境変数を利用して、GATES 用 Audience の JWT を取得する。
3. `umask 077` を適用し、アクセスを制限した一時ディレクトリ内に、GATES 用 OIDC JWT を含むリクエストファイルと Installation Access Token を受け取るレスポンスファイルを用意する。正常終了・異常終了のいずれでも一時ファイルを削除する後処理を設定する。
4. リクエスト JSON を作成し、AWS CLI `lambda invoke` を同期実行する。リクエストは `--payload fileb://...` で渡し、レスポンス本文は AWS CLI の必須出力ファイルへ保存する。CLI の標準出力に返る呼び出しメタデータは、トークンを含むレスポンス本文とは区別する。
5. AWS CLI の終了コード、同期 Invoke の `StatusCode`、`FunctionError` の有無、レスポンス JSON の `ok` を確認する。成功時だけ取得したトークンを GitHub Actions のマスク対象に登録し、トークン、有効期限、対象、実効権限を内部ステップの `GITHUB_OUTPUT` へ書き込む。レスポンスの `repositories` 配列と `permissions` オブジェクトは入力と同じカンマ区切りへ変換する。トークン本体を通常出力しない。
6. 後処理で入出力の一時ファイルを削除する。`set -x`、AWS CLI の `--debug`、`--log-type Tail` を利用しない。

Composite Action は入力値を [Lambda Request](lambda.md#2-request) に従って変換し、[Lambda Responses](lambda.md#3-responses) に従って結果を判定します。`ok` が `false` のときはエラーコードを利用者に示してエラー終了し、GATES 用 OIDC JWT、AWS 一時認証情報、発行されたトークンをログへ出しません。一時ファイルの削除は処理後の残留リスクを減らす対策であり、ランナーで動作する信頼できないコードから資格情報を保護するものではありません。

## 4. Usage

### 4.1 Workflow Example

`example-org/gates-action` は仮の名前であり、`<PINNED_COMMIT>` は実際の固定コミット SHA に置き換えます。アクセス先は Contents 読み取り権限が必要な Private Repository とし、`README.md` が存在する前提です。

```yaml
name: Read another repository
on: workflow_dispatch

jobs:
  read-repository:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
    steps:
      - name: Issue token
        id: gates
        uses: example-org/gates-action@<PINNED_COMMIT>
        with:
          repositories: example-org/repo-a,example-org/repo-b
          permissions: contents:read

      - name: Access repository
        env:
          GH_TOKEN: ${{ steps.gates.outputs.token }}
        run: gh api repos/example-org/repo-a/contents/README.md
```

## 5. Technology Decisions

| 用途 | 採用技術・方針 | 理由 |
|---|---|---|
| AWS 一時認証情報の取得 | `aws-actions/configure-aws-credentials` | GitHub OIDC と AWS STS の処理を公式 Action に委譲 |
| Lambda Invoke | AWS CLI | SigV4 署名と同期呼び出しを AWS CLI に委譲 |
| JWT 取得と入力変換 | GitHub Actions の OIDC 取得機能、Shell、JSON 処理ツール | GitHub の標準機構を利用して Payload を組み立てる |

外部 Action は固定コミット SHA を参照します。必要な AWS CLI、JSON 処理ツール、GitHub Actions の OIDC 取得機能を利用できるランナーを前提とします。

## 6. Testability

入力形式の異常、AWS 認証の失敗、AWS CLI の終了コード、`StatusCode` と `FunctionError`、`ok:false`、Action Output への転送（レスポンスの配列・オブジェクトからカンマ区切りへの変換を含む）を検証します。GATES 用 OIDC JWT を含むリクエストファイルと発行トークンを含むレスポンスファイルについて、権限設定、正常系・異常系での後処理、ログへの資格情報非出力、同一ジョブ内の後続ステップからの利用を確認します。受け入れ条件は [Requirements](../spec/requirements.md) を正本とします。

## 7. Sources

- [GitHub — Creating a composite action](https://docs.github.com/en/actions/tutorials/create-actions/create-a-composite-action)
- [GitHub — Metadata syntax for composite action outputs](https://docs.github.com/en/actions/reference/workflows-and-actions/metadata-syntax)
- [GitHub — Workflow commands / GITHUB_OUTPUT](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)
- [GitHub — OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)
- [AWS — configure-aws-credentials](https://github.com/aws-actions/configure-aws-credentials)
- [AWS — AWS CLI Lambda Invoke](https://docs.aws.amazon.com/cli/latest/reference/lambda/invoke.html)
- [GitHub — REST API endpoints for repository contents](https://docs.github.com/en/rest/repos/contents)
