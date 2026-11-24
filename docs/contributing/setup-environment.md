# Testing Environment Setup

GATES の動作を検証するには、トークンの発行元となる GitHub App と、デプロイした GATES が必要です。
検証環境の構築は、運用者向けの [Deployment](../user-guide/deployment.md) の手順に従います。
本ドキュメントには、検証環境に固有の値と手順の差分だけを書きます。

## 1. GitHub App

検証用の GitHub App は、開発者の個人アカウントに作り、そのアカウントを許可オーナーにします。
Deployment の §2.1 のうち、運用者が決める項目は次の値にします。
ほかの項目は Deployment のとおりです。

| 項目 | 設定値 |
|---|---|
| GitHub App name | `Testing GATES` |
| Description | `A GitHub App for testing GATES (GitHub App Token Exchange System), a system that issues GitHub App installation access tokens to GitHub Actions workflows.` |
| Homepage URL | `https://example.com` |

Deployment の Repository permissions は、[E2E Tests](e2e-tests.md) の事例が使う次の水準にします。

| 権限 | 水準 | 使う事例 |
|---|---|---|
| `Contents` | `Read-only` | `contents:read` の発行 |
| `Issues` | `Read and write` | 書き込みの水準の発行、複数の権限の発行、水準の包含 |
| `Commit statuses` | `Read-only` | GitHub App に付与されていて、発行上限に含まれない `statuses:read` の要求の拒否 |
| `Pull requests` | 付与しない | 発行上限に含まれて、付与されていない `pull_requests:read` の要求の拒否 |
| `Single file` | `Read-only`、パスは `.github/gates-policy.yml` だけ | リポジトリポリシーの読み取り。`single_file:read` の要求がリポジトリポリシーだけで拒否されること |

`Single file` のパスを `.github/gates-policy.yml` のほかに加えると、`single_file:read` のトークンで読めるファイルが増えるため、加えません。

ほかの Repository permissions、Organization permissions、Account permissions は付与しません。
書き込みは `Issues` だけにし、`Contents`、`Pull requests`、`Workflows`、`Administration`、`Secrets`、`Actions` の書き込みは付与しません。
これらはコードやワークフロー、リポジトリの設定に作用し、インストール先のリポジトリも、許可オーナーのどのワークフローからでも、発行したトークンで操作されるためです。
`Issues` の書き込みも、Issue のイベントで動くワークフローを起動できるため、インストール先のリポジトリにそのワークフローを置きません。

## 2. Repositories to Install

Deployment のインストール対象には、[E2E Tests](e2e-tests.md) が使うリポジトリだけを加えます。
E2E テストは、インストール対象の 3 つの非公開リポジトリと、インストール対象に含めない許可オーナーの公開リポジトリを使います。
3 つ目のリポジトリは、リポジトリポリシーのない対象への要求の拒否だけに使います。
どのリポジトリをどの Variables に登録するかは、E2E Tests の Setup にあります。

GATES を呼び出す試験用のリポジトリそのものは、インストール対象に含めません。
E2E テストは、試験用のリポジトリを要求に含めたときに発行が拒否されることも確かめるためです。

## 3. Deployment

Deployment の §1 から §7 に従って、AWS リソースを適用し、鍵素材をインポートし、接続情報を試験用のリポジトリに登録します。
`terraform.tfvars` の発行上限（`issuable_permissions`）と保護対象の一覧（`protected_repositories`）は、E2E テストのすべての事例が同じ値で動くよう、次の値に固定します。
E2E テストを動かすたびに値を変えて適用することはしません。

```hcl
issuable_permissions = [
  "contents:read",
  "metadata:read",
  "issues:write",
  "pull_requests:read",
  "single_file:read",
  "actions:read",
]
```

- **`issues:write`**：`issues` は `write` だけを含め、`issues:read` の要求が水準の包含で発行されることを確かめる
- **`contents:read`**：`contents` は `read` だけを含め、`contents:write` の要求が GitHub に届く前に拒否されることを確かめる
- **`pull_requests:read`**：GitHub App に付与しないが発行上限に含め、GATES の認可で止めずに GitHub に届け、GitHub App の権限による拒否を確かめる
- GitHub App に付与した `statuses:read` は発行上限に含めず、発行上限による拒否を確かめる
- **`single_file:read`**：保護対象の一覧は許し、1 つ目の対象のリポジトリポリシーは許さない権限として、リポジトリポリシーだけによる拒否を確かめる
- **`actions:read`**：GitHub App に付与しないが発行上限に含め、1 つ目の対象のリポジトリポリシーは許し、保護対象の一覧は許さない権限として、保護対象の一覧だけによる拒否を確かめる

保護対象の一覧は、1 つ目の対象を保護対象にして試験用のリポジトリを許可 Caller にし、ほかは一覧にだけある実在しないリポジトリの名前にします。

```hcl
protected_repositories = <<-EOT
  repositories:
    - id: "<1 つ目の対象の Repository ID>"
      name: <1 つ目の対象のリポジトリ名>
      callers:
        - repository_id: "<試験用のリポジトリの Repository ID>"
          repository_name: <試験用のリポジトリのリポジトリ名>
          permissions:
            contents: read
            metadata: read
            issues: write
            pull_requests: read
            single_file: read
    - id: "9000000001"
      name: gates-e2e-no-callers
      callers: []
    - id: "9000000002"
      name: gates-e2e-other-caller
      callers:
        - repository_id: "9000000003"
          repository_name: gates-e2e-other-caller-source
          permissions:
            contents: read
    - id: "9000000004"
      name: gates-e2e-caller-name-mismatch
      callers:
        - repository_id: "<試験用のリポジトリの Repository ID>"
          repository_name: <試験用のリポジトリのリポジトリ名>-renamed
          permissions:
            contents: read
    - id: "9000000005"
      name: gates-e2e-caller-id-mismatch
      callers:
        - repository_id: "9000000006"
          repository_name: <試験用のリポジトリのリポジトリ名>
          permissions:
            contents: read
    - id: "9000000007"
      name: gates-e2e-read-only
      callers:
        - repository_id: "<試験用のリポジトリの Repository ID>"
          repository_name: <試験用のリポジトリのリポジトリ名>
          permissions:
            contents: read
            issues: read
EOT
```

Repository ID は `gh api repos/<オーナー>/<リポジトリ> --jq .id` で調べ、引用符で囲みます。
`gates-e2e-` で始まる名前のリポジトリは、許可オーナーに作りません。
実在しない名前の保護対象への要求は、発行の前の認可で拒否され、GitHub 側にリポジトリが要らないためです。
`9000000001` から `9000000007` の ID は、インストール先の 2 つのリポジトリと試験用のリポジトリの ID でなければ、どの値でも構いません。

1 つ目の対象の許可 Caller の上限には、E2E テストの事例が 1 つ目の対象に要求する権限をすべて含めます。
上限から外れた権限を要求する事例は、保護対象の条件で拒否されて結果が変わるためです。
2 つ目の対象は保護対象にせず、保護対象でない対象への要求と、保護対象と混ぜた要求を確かめます。
事例と一覧の対応は [E2E Tests](e2e-tests.md) の Success Cases と Protected Repositories Cases にあります。

リポジトリポリシーは適用する設定に固定します。

```hcl
repository_policy_enforcement = true
```

1 つ目と 2 つ目の対象の既定ブランチには、試験用のリポジトリを許可 Caller にしたリポジトリポリシーを、`.github/gates-policy.yml` に置きます。
3 つ目の対象には置きません。
1 つ目の対象の権限の上限は、保護対象の一覧の許可 Caller の上限と違う組にし、片方だけが許す権限を作ります。

```yaml
# 1 つ目の対象
callers:
  - repository_id: "<試験用のリポジトリの Repository ID>"
    repository_name: <試験用のリポジトリのリポジトリ名>
    permissions:
      contents: read
      metadata: read
      issues: write
      pull_requests: read
      actions: read
```

```yaml
# 2 つ目の対象
callers:
  - repository_id: "<試験用のリポジトリの Repository ID>"
    repository_name: <試験用のリポジトリのリポジトリ名>
    permissions:
      contents: read
      metadata: read
      issues: read
```

1 つ目の対象のリポジトリポリシーは、E2E テストの事例が 1 つ目の対象に要求する権限のうち、`single_file:read` 以外をすべて含めます。
`pull_requests:read` は、Payload の拒否の事例を GitHub の `422` まで届けるために要ります。
2 つ目の対象の `issues` を `read` にするのは、`issues:write` の要求をリポジトリポリシーの権限の上限で拒否させるためです。
事例とリポジトリポリシーの対応は [E2E Tests](e2e-tests.md) の Repository Policy Cases にあります。

接続情報のほかに E2E テストが使う Variables は、E2E Tests の Setup にあります。

Deployment の最初の発行の確認には、E2E テストの `_e2e-success.yml` を使えます。

## 4. Key Rotation

[Key Rotation](../user-guide/rotate-key.md) の手順の中で発行を確かめるときは、E2E テストの `_e2e-success.yml` を使います。
