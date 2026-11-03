# Testing Environment Setup

GATES の動作を検証するには、トークンの発行元となる GitHub App と、その Private Key、GATES の AWS リソースが必要です。
本ドキュメントでは、検証用の GitHub App を個人アカウントに作成し、インストールして、Private Key を発行するまでと、AWS リソースを Terraform で適用する手順を扱います。

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

GitHub App の設定ページ（[1.2](#12-installing-the-github-app) と同じページ）で、`Credentials` → `Key pairs` → `New key` をクリックします。
`<app-name>.<date>.private-key.pem` という名前のファイルがダウンロードされるので、リポジトリの外のディレクトリに保存します。

ダウンロードした鍵が正しいかは、次のコマンドで出力される SHA256 フィンガープリントを、設定ページに表示されたフィンガープリントと比べて確かめます。

```bash
openssl rsa -in /path/to/private-key.pem -pubout -outform DER \
  | openssl sha256 -binary | openssl base64
```

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

- **`gates_audience`**：運用者が持つ名前空間の、GATES 専用だと分かる URI にする（例：GATES のコードを置くリポジトリの URL `https://github.com/<owner>/<repo>`）。呼び出し元のリポジトリの URL にはしない。Audience はすべての呼び出し元に共通の固定値であるため。AWS 用 OIDC JWT の Audience `sts.amazonaws.com` と、GitHub の既定の Audience（`https://github.com/<owner>`）は、それぞれの Audience の OIDC JWT で GATES の認証を通せるようになるため、変数の検証で拒否される
- **`kms_key_spec`**：インポートする GitHub App Private Key の鍵長に合わせる。鍵長は `openssl rsa -in /path/to/private-key.pem -text -noout` の先頭行で確かめる
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

### 3.3 Local Terraform Files

State、Plan、`terraform.tfvars` は `terraform/local/` に置かれます。
このディレクトリは Git の管理外で、静的解析のコンテナからも読めません。

`terraform/local/` は消さずに残します。
State を失うと、Terraform は作成済みのリソースを管理できなくなります。
その状態で適用し直すと、同じ名前の IAM ポリシーや IAM Role の作成が `EntityAlreadyExists` で失敗します。

## 4. Sources

- [GitHub — Registering a GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app)
- [GitHub — Installing your own GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-your-own-github-app)
- [GitHub — Managing private keys for GitHub Apps](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps)
- [AWS — Configuring reserved concurrency for a function](https://docs.aws.amazon.com/lambda/latest/dg/configuration-concurrency.html)
