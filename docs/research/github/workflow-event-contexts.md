# GitHub Workflow Event Contexts

本ドキュメントでは、起動イベントと Environment の異なるジョブが、GATES の Docker Action でトークンを得たときに、IAM Role の引き受け、発行、GATES の監査記録がどうなったかを記述します。
GitHub Actions の実環境で実行し、ワークフローのログ、CloudTrail のイベント履歴、GATES の関数のロググループから読んだ値を掲載しています。

- 調査日: **2026/11/14**
- 検証環境: 個人アカウントのプライベートリポジトリ。immutable `sub` を使う
- 要求: 対象 1 件、`contents:read`。Docker Action は受け入れの対象の commit で参照した

オーナー名は `octocat`、リポジトリ名は `example-repo`、要求の対象は `octocat/example-target`、Owner ID は `583231`、Repository ID は `1234567890`、Pull Request の番号は `1`、ブランチ名は `example-branch` に置き換えています。
commit SHA、ワークフロー実行の ID、相関 ID は記録していません。

## Trust Policy

引き受けた IAM Role の Trust Policy は、`sub` の `REPO@REPO-ID` より後ろを制限しません。

```json
{
  "StringEquals": { "token.actions.githubusercontent.com:aud": "sts.amazonaws.com" },
  "StringLike": { "token.actions.githubusercontent.com:sub": "repo:octocat@583231/*" }
}
```

## Results

ワークフローは既定のブランチ（`main`）に置き、Pull Request は同じリポジトリのブランチ `example-branch` から `main` に向けて作りました。
`pull_request_target` は fork を要さず、同じリポジトリのブランチからの Pull Request でも起動します。
fork からの Pull Request で起動した `pull_request_target` と `pull_request` は試していません（[Fork Pull Requests](#fork-pull-requests)）。
`pull_request_target` のジョブは Pull Request のコードをチェックアウトしていません。

どの文脈でも、IAM Role の引き受けと発行は成功し、トークンで要求の対象の `README.md` を読めて（`200`）、post-entrypoint の失効は `204` でした。

| 文脈 | 起動の方法 | AWS 用 OIDC JWT の `sub` の後半 | 結果 |
|---|---|---|---|
| `pull_request` | Pull Request の作成 | `pull_request` | 成功 |
| `pull_request_target` | 同じ Pull Request の作成 | `pull_request` | 成功 |
| Environment を使うジョブ | `workflow_dispatch`。ジョブに `environment: testing` | `environment:testing` | 成功 |
| `workflow_run` | 別のワークフローの完了 | `ref:refs/heads/main` | 成功 |

`schedule` も試しましたが、一度も起動しませんでした。
日付と時刻を固定した cron（`*/10 5-6 14 11 *` など）を既定のブランチに置き、約 3 時間にわたって約 20 回の枠を設けました。最後は `schedule` だけを持つワークフローに分けましたが、起動した実行はありませんでした。
同じファイルのほかのイベントは同じ時間帯に動いており、GitHub が `schedule` の実行を遅らせたか落としたと推測しますが、原因は確かめていません。
GitHub は `schedule` の `GITHUB_REF` を既定のブランチとしているため、`sub` の後半は `ref:refs/heads/main` になる見込みです。

`sub` の後半は、CloudTrail の `AssumeRoleWithWebIdentity` の記録の `userName` から読みました。
`pull_request_target` の `sub` は `pull_request` と同じ形で、`sub` から 2 つのイベントを区別できません。
`workflow_run` の `sub` は、既定のブランチへの push や `workflow_dispatch` と同じ形です。

## Audit Records

監査記録は、どの文脈でも相関 ID ごとに 1 行で、`result` は `success`、`caller.run_id` はワークフロー実行の ID と一致しました。
ワークフロー起動属性は次のとおりで、`actor` と `actor_id` はどの文脈でもオーナーでした。

| 文脈 | `event_name` | `ref` | `sha` | `workflow_ref` | `environment` |
|---|---|---|---|---|---|
| `pull_request` | `pull_request` | `refs/pull/1/merge` | マージコミット | `.../event-contexts.yml@refs/pull/1/merge` | なし |
| `pull_request_target` | `pull_request_target` | `refs/heads/main` | `main` の先端 | `.../event-contexts.yml@refs/heads/main` | なし |
| Environment を使うジョブ | `workflow_dispatch` | `refs/heads/main` | `main` の先端 | `.../event-contexts.yml@refs/heads/main` | `testing` |
| `workflow_run` | `workflow_run` | `refs/heads/main` | `main` の先端 | `.../event-contexts.yml@refs/heads/main` | なし |

`job_workflow_ref` は、どの文脈でも `workflow_ref` と同じ値でした。

監査記録から、どのイベントの、どの ref の、どのワークフローが得たトークンかは、どの文脈でも読めます。
一方、`pull_request_target` と `workflow_run` では、ジョブに信頼できない入力を持ち込み得る起点が監査記録に残りません。
`pull_request_target` の記録には、どの Pull Request で起動したかを示す項目がなく、`ref` と `sha` は既定のブランチの値です。
同じ Pull Request の `pull_request` の記録とは、`ref` の Pull Request の番号で対応がつくだけです。
`workflow_run` の記録には、起点のワークフロー実行を示す項目がありません。
どちらも、`caller.run_id` で GitHub のワークフロー実行をたどれる間は、そこから起点を調べられます。

ワークフローのログでは、`pull_request_target` のジョブの `github.head_ref` は `example-branch`、`github.base_ref` は `main` でした。
GitHub は OIDC JWT の `head_ref` と `base_ref` の Claim を、Pull Request の元ブランチと宛先ブランチと説明していますが、GATES はどちらも監査記録に写していません。
OIDC JWT の値は記録していないため、`pull_request_target` のジョブで、この 2 つの Claim がワークフローのログと同じ値だったかは確かめていません。

## Fork Pull Requests

fork からの Pull Request は、`pull_request` と `pull_request_target` のどちらも実施していません。
プライベートリポジトリを fork するには、許可オーナーとは別のアカウントを共同作業者に招く必要があるためです。
代わりに、GitHub の公式資料で次の記述を確かめました。

- fork からの Pull Request が起動した `pull_request_target` 以外のイベントでは、「Send write tokens to workflows from pull requests」を選んでいなければ、`GITHUB_TOKEN` の write の権限が read に下げられる（Workflow syntax の `permissions`）
- `GITHUB_TOKEN` 以外の Secrets は、fork から起動したワークフローに渡らない（Events that trigger workflows）
- プライベートリポジトリでは、fork からの Pull Request のワークフローは、「Run workflows from fork pull requests」を有効にした場合だけ動く。「Send write tokens to workflows from pull requests」は fork の Pull Request に write の `GITHUB_TOKEN` を、「Send secrets to workflows from pull requests」はすべての Secrets を渡す（Managing GitHub Actions settings for a repository）
- `pull_request_target` のジョブは、fork からの Pull Request でも base のリポジトリの `GITHUB_TOKEN` と Secrets を受け取り、`GITHUB_TOKEN` は public な fork からでも read/write の権限を持つ（Securely using pull_request_target、Workflow syntax の `permissions`）
- `workflow_run` で起動したワークフローは、起点のワークフローが持たなくても Secrets と write のトークンを持つ（Events that trigger workflows）

OIDC JWT の要求には `id-token: write` が要ります（Workflow syntax の `permissions`）。
資料は `id-token` を名指ししていませんが、`id-token` は `GITHUB_TOKEN` の権限の 1 つであるため、上の記述から次のように見込みます。
fork からの `pull_request` は既定では OIDC JWT を要求できず、`pull_request_target` と `workflow_run` は fork が起点でも、宣言した `id-token: write` で要求できる見込みです。
この見込みは実環境で確かめていません。

この見込みのとおりなら、Pull Request を作れるアカウントが GATES からトークンを得る経路は、プライベートリポジトリで上の設定を有効にした場合の `pull_request` と、`pull_request_target` や `workflow_run` のジョブが Pull Request のコードや成果物を実行した場合です。

## Workflow File

試験に使用したワークフローファイルは次のとおりです。
`workflow_run` の起点には、作業ブランチへの push で動き、何もしないワークフロー（`name: Event Contexts Trigger`）を使いました。
接続情報の Secret と Variables の名前は、[Testing Environment Setup](../../contributing/setup-environment.md) の登録に従います。
ジョブのログの資格情報の検索は、E2E テストの `_e2e-reusable-credential-leak.yml` と同じ検索を、スキップしたジョブを除いて行いました。ここでは省いています。

```yaml
name: Event Contexts
on:
  pull_request:
    branches:
      - main
  pull_request_target:
    branches:
      - main
  workflow_dispatch:
  workflow_run:
    workflows:
      - Event Contexts Trigger
    types:
      - completed
permissions: {}
concurrency:
  group: gates-e2e
  cancel-in-progress: false
  queue: max
jobs:
  issue:
    if: ${{ github.event_name != 'workflow_dispatch' }}
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      id-token: write
      contents: read
    steps:
      - name: Issue token
        id: gates
        uses: tmknom/gates/.github/actions/issue-token@<commit SHA>
        with:
          repositories: ${{ vars.GATES_TARGET_PRIMARY }}
          permissions: contents:read
          role-arn: arn:aws:iam::${{ secrets.GATES_AWS_ACCOUNT_ID }}:role/${{ vars.GATES_ROLE_NAME }}
          function-name: ${{ vars.GATES_FUNCTION_NAME }}
          aws-region: ${{ vars.GATES_AWS_REGION }}
          audience: ${{ vars.GATES_AUDIENCE }}
      - name: Use token
        shell: bash
        env:
          GH_TOKEN: ${{ steps.gates.outputs.token }}
          REPOSITORY: ${{ vars.GATES_TARGET_PRIMARY }}
        run: |
          set -euo pipefail
          status="$(gh api --include "repos/${REPOSITORY}/contents/README.md" 2> /dev/null | head -n 1 | awk '{print $2}' || true)"
          echo "README.md: ${status}"
          [[ "${status}" == "200" ]]
  issue-environment:
    if: ${{ github.event_name == 'workflow_dispatch' }}
    runs-on: ubuntu-latest
    timeout-minutes: 10
    environment: testing
    permissions:
      id-token: write
      contents: read
    steps:
      # issue と同じステップ
```

## References

- [GitHub — Events that trigger workflows](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows)
- [GitHub — Securely using pull_request_target](https://docs.github.com/en/actions/reference/security/securely-using-pull_request_target)
- [GitHub — Workflow syntax for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#permissions)
- [GitHub — Managing GitHub Actions settings for a repository](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository#enabling-workflows-for-forks-of-private-repositories)
- [GitHub — OpenID Connect reference](https://docs.github.com/en/actions/reference/security/oidc)
