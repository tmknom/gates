# Testing Environment Setup

GATES の動作を検証するには、トークンの発行元となる GitHub App と、その Private Key、GATES の AWS リソースが必要です。
本ドキュメントでは、検証用の GitHub App を個人アカウントに作成してインストールする手順と、AWS リソースを Terraform で適用する手順を扱います。

## 1. GitHub App

### 1.1 Creating the GitHub App

`Settings` → `Developer settings` → `GitHub Apps` → `New GitHub App` の順に開き、GitHub App の作成ページで次のように設定します。

| 項目 | 設定値 |
|---|---|
| GitHub App name | `Testing GATES` |
| Description | `A GitHub App for testing GATES (GitHub App Token Exchange System), a system that issues GitHub App installation access tokens to GitHub Actions workflows.` |
| Homepage URL | `https://example.com` |
| Webhook | `Active` のチェックを外す |
| Repository permissions | `Contents` を `Read-only` にする |
| Account permissions | 何も選択しない |
| Where can this GitHub App be installed? | `Only on this account` を選ぶ |

最後に `Create GitHub App` をクリックすると、GitHub App が作成されます。

> [!NOTE]
> `Contents` を `Read-only` にすると、`Metadata` も自動的に `Read-only` になり、`Mandatory` ラベルが付きます。

### 1.2 Installing the GitHub App

`Settings` → `Developer settings` → `GitHub Apps` の順に開き、作成した GitHub App の `Edit` から設定ページを開きます。
左のメニューの `Install App` を開き、自分のアカウントの `Install` をクリックします。

インストール先のリポジトリは `Only select repositories` を選び、`Select repositories` で検証に使うリポジトリだけを追加します。
最後に `Install` をクリックすると、インストールが完了します。

## 2. GitHub App Private Key

GitHub App Private Key は、AWS リソースを適用した後に、[Key Material Import](import-key-material.md) の手順の中で生成します。
生成した鍵は KMS Key にインポートし、手元のコピーは削除します。
先に生成すると、インポートするまでのあいだ、秘密鍵が手元に残り続けるためです。

## 3. AWS Infrastructure Deployment

GATES の AWS リソースは、運用者が手元から Terraform で適用します。
Terraform はすべて `make` のターゲットからコンテナで実行し、手元の `terraform` は使いません。

### 3.1 Prerequisites

Plan の前に次を満たしておきます。

- Role の作成に特定の Permissions Boundary を求めるアカウントでは、`gates-permissions-boundary` の ARN を許可の対象に加えてある
- アカウントに `https://token.actions.githubusercontent.com` の IAM OIDC Provider があり、Client ID に `sts.amazonaws.com` を含む。Terraform はこの Provider を参照するため、ない間は GATES と無関係な変更でも `make tf-plan` が失敗する
- `terraform/terraform.tfvars.example` を `terraform/local/terraform.tfvars` にコピーし、値を設定してある
- 一時認証情報（`AWS_ACCESS_KEY_ID`、`AWS_SECRET_ACCESS_KEY`、`AWS_SESSION_TOKEN`）をシェルの環境変数に設定してある。`make tf-plan` と `make tf-apply` は、`AWS_SESSION_TOKEN` のない長期のアクセスキーでは起動しない

`terraform.tfvars` の値のうち、次の 3 つは適用の前に確かめて決めます。

- **`gates_audience`**：GATES 専用の、URL でない文字列にする（例：`<owner>-gates`）。`sts.amazonaws.com`、`https://github.com`、`https://github.com/` で始まる値は、変数の検証で拒否される（[Domain Model](../design/domain.md#53-invariants)）
- **`kms_key_spec`**：インポートする GitHub App Private Key の鍵長に合わせる。秘密鍵は適用の後に生成するため、検証用 App で生成した鍵の鍵長（2048 bit）から `RSA_2048` とする
- **`lambda_reserved_concurrency`**：`aws lambda get-account-settings` の `UnreservedConcurrentExecutions` から予約する数を引いても、Lambda が求める未予約の下限（100）を下回らない値にする

`lambda_package_path` は `terraform.tfvars` に書きません。
`make tf-plan` がデプロイパッケージを作り、そのパスを渡します。

### 3.2 Plan and Apply

`make tf-plan` を実行し、出力で対象のアカウント、リージョン、変更の内容を確かめます。
`make tf-plan` は実行のたびにデプロイパッケージを作り直し、Plan を `terraform/local/plan.tfplan` に保存します。

```bash
make tf-plan
```

KMS Key には `prevent_destroy` を設定しているため、KMS Key の置換や削除を含む Plan は `Instance cannot be destroyed` のエラーで止まり、`make tf-plan` は保存した Plan を捨てます。
このエラーが出たら、エラーの前に表示される変更の内容から、置換の原因（`forces replacement` の付いた属性）を調べます。

Plan の内容に問題がなければ、`make tf-apply` で保存した Plan を適用します。
`make tf-apply` は、デプロイパッケージが Plan のときと同じであることを確かめてから適用します。

```bash
make tf-apply
```

適用の後に、もう一度 `make tf-plan` を実行し、`No changes` になることを確かめます。

初めての適用では、KMS Key は鍵素材を持たない `PendingImport` の状態で作られます。
`make tf-apply` が出力する `kms_key_arn` を指定して次のコマンドを実行し、`Origin` が `EXTERNAL`、`KeyUsage` が `SIGN_VERIFY`、`KeyState` が `PendingImport`、`KeySpec` が `kms_key_spec` の値であることを確かめます。

```bash
aws kms describe-key --key-id <kms_key_arn>
```

続けて、[Key Material Import](import-key-material.md) の手順で、GitHub App Private Key を生成して KMS Key にインポートします。

### 3.3 Local Terraform Files

State、Plan、`terraform.tfvars` は `terraform/local/` に置かれます。
このディレクトリは Git の管理外で、静的解析のコンテナからも読めません。

`terraform/local/` は消さずに残します。
State を失うと、Terraform は作成済みのリソースを管理できなくなります。
その状態で適用し直すと、同じ名前の IAM ポリシーや IAM Role の作成が `EntityAlreadyExists` で失敗します。

### 3.4 Connection Information

GATES を呼び出すワークフローのために、接続情報を GitHub の Secrets と Variables に登録します（[GitHub Actions Design](../design/github-actions.md#21-inputs-and-outputs)）。
AWS アカウント ID は GATES を適用したアカウントの ID で、適用に使った認証情報で `aws sts get-caller-identity --query Account --output text` を実行すると確かめられます。
Role 名と関数名は、`make tf-output` が State から表示する `github_actions_role_name` と `lambda_function_name` の値です。
リージョンと Audience は、`terraform/local/terraform.tfvars` の `region` と `gates_audience` の値です。

```bash
make tf-output
```

登録する単位は、許可オーナーの種別で決まります。
オーナーが Organization なら、Organization 単位に一度だけ登録します。
`--visibility` を付けないと、登録した値は private リポジトリからだけ参照できます。
参照できるリポジトリをさらに絞るときは、`--visibility selected` と `--repos` で対象を指定します。

```bash
gh secret set GATES_AWS_ACCOUNT_ID --org <owner>
gh variable set GATES_ROLE_NAME --org <owner> --body <github_actions_role_name>
gh variable set GATES_FUNCTION_NAME --org <owner> --body <lambda_function_name>
gh variable set GATES_AWS_REGION --org <owner> --body <region>
gh variable set GATES_AUDIENCE --org <owner> --body <gates_audience>
```

オーナーがユーザーアカウントなら、GATES を呼び出す各リポジトリに登録します。
Organization でも、GitHub Free の private リポジトリと、ポリシーで利用を制限したリポジトリは Organization 単位の値を参照できないため、同じように個別に登録します。

```bash
gh secret set GATES_AWS_ACCOUNT_ID --repo <owner>/<repo>
gh variable set GATES_ROLE_NAME --repo <owner>/<repo> --body <github_actions_role_name>
gh variable set GATES_FUNCTION_NAME --repo <owner>/<repo> --body <lambda_function_name>
gh variable set GATES_AWS_REGION --repo <owner>/<repo> --body <region>
gh variable set GATES_AUDIENCE --repo <owner>/<repo> --body <gates_audience>
```

`gh secret set` は `--body` を付けないと値の入力を求めるため、AWS アカウント ID をコマンドラインに書かずに済みます。

## 4. Sources

- [GitHub — Registering a GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app)
- [GitHub — Installing your own GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-your-own-github-app)
- [GitHub — Managing private keys for GitHub Apps](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps)
- [GitHub CLI — gh secret set](https://cli.github.com/manual/gh_secret_set)
- [GitHub CLI — gh variable set](https://cli.github.com/manual/gh_variable_set)
- [AWS — Configuring reserved concurrency for a function](https://docs.aws.amazon.com/lambda/latest/dg/configuration-concurrency.html)
