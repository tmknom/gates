# Deployment

運用者に向けて、GATES を初めてデプロイし、最初のトークンの発行を確かめるまでの手順を示します。
上から順に進めると、許可オーナーのワークフローが Action でトークンを受け取れるようになります。
GATES の仕組みと用語は [GATES Overview](overview.md) にあります。

## 1. Order of the Work

| 節 | 作業 | 場所 |
|---|---|---|
| §2 | 前提を確かめる | 手元、AWS、GitHub |
| §3 | GitHub App を作成し、許可オーナーにインストールする | GitHub |
| §4 | GATES のソースを取得し、配布物を検証する | 手元 |
| §5 | `terraform.tfvars` を設定する | 手元 |
| §6 | Terraform で AWS リソースを作る | 手元、AWS |
| §7 | GitHub App Private Key を生成し、KMS Key にインポートする | GitHub、手元、AWS |
| §8 | 接続情報を GitHub に登録する | GitHub |
| §9 | 最初の発行を確かめる | GitHub、AWS |

一部の環境でだけ要る作業は §10 にあります。
§4 以降のコマンドは、GATES のリポジトリのルートで、同じシェルから実行します。

## 2. Prerequisites

### 2.1 Local Tools

| ツール | 使う場面 |
|---|---|
| Git | GATES のリポジトリを取得する |
| `make` | デプロイと鍵素材のインポートを実行する |
| Docker（Docker Compose を含む） | Terraform をコンテナの中で実行する。手元の `terraform` は使わない |
| Go | Lambda のデプロイパッケージをビルドする |
| `zip`、`shasum` | デプロイパッケージを作り、確かめる |
| AWS CLI、`jq`、`xxd`、OpenSSL 3 以降 | 鍵素材をインポートし、結果を確かめる。macOS 標準の LibreSSL では動かない |
| GitHub CLI | 配布物を検証し、接続情報を登録する。`gh auth login` でログインしておく |

### 2.2 AWS Credentials

AWS の一時認証情報（`AWS_ACCESS_KEY_ID`、`AWS_SECRET_ACCESS_KEY`、`AWS_SESSION_TOKEN`）をシェルの環境変数に設定します。
`make tf-plan`、`make tf-apply`、`make kms-import-key-material` は、`AWS_SESSION_TOKEN` のない長期のアクセスキーでは動きません。
AWS CLI のリージョン（`AWS_REGION`）は、§5 で `region` に設定するリージョンにします。

一時認証情報の主体には、次の権限が要ります。

- IAM Role とポリシー、KMS Key と Key Policy、Lambda の関数、CloudWatch Logs のロググループの作成と変更
- KMS Key への鍵素材のインポート
- 確認に使う読み取り（`iam:GetOpenIDConnectProvider`、`kms:DescribeKey`、`lambda:GetFunction`、`logs:FilterLogEvents`）

### 2.3 IAM OIDC Provider

AWS アカウントに、GitHub Actions の OIDC JWT を信頼する IAM OIDC Provider が要ります。
GATES の Terraform は Provider を作らず、アカウントにあるものを使います。
次のコマンドで確かめます。

```bash
aws iam get-open-id-connect-provider \
  --open-id-connect-provider-arn arn:aws:iam::<AWS アカウント ID>:oidc-provider/token.actions.githubusercontent.com \
  --query ClientIDList
```

出力の配列に `sts.amazonaws.com` があれば、満たしています。
`NoSuchEntity` のエラーになったときは、§10.1 で作ります。

### 2.4 OIDC JWT of the Repositories

GATES を呼び出すリポジトリの OIDC JWT は、呼び出し元を示す `sub` が、`repo:<オーナー名>@<Owner ID>/<リポジトリ名>@<Repository ID>:...` のように不変の ID を含む形式でなければなりません。
名前だけの形式のリポジトリからの呼び出しは、AWS が拒否します。
新しいリポジトリは既定でこの形式ですが、古いリポジトリは名前だけの形式のことがあります。

リポジトリごとに、次のコマンドで確かめます。

```bash
gh api repos/<オーナー名>/<リポジトリ名>/actions/oidc/customization/sub
```

出力の `use_immutable_subject` が `false` で、`sub_claim_prefix` に `@` がなければ、名前だけの形式です。

```json
{"use_default":true,"use_immutable_subject":false,"sub_claim_prefix":"repo:octocat/example-legacy-repo"}
```

その場合は、リポジトリの管理者が次のコマンドで切り替えます。
許可オーナーが Organization なら、Organization の設定（REST API の `use_immutable_subject`）でまとめて切り替えられます。

```bash
gh api --method PUT repos/<オーナー名>/<リポジトリ名>/actions/oidc/customization/sub \
  -F use_default=true -F use_immutable_subject=true
```

`sub` の形式をカスタマイズして、先頭がこの形式でなくなったリポジトリからも呼び出せません。

## 3. Creating and Installing the GitHub App

### 3.1 Creating the GitHub App

許可オーナーがユーザーアカウントなら、`Settings` → `Developer settings` → `GitHub Apps` → `New GitHub App` を開きます。
Organization なら、Organization の `Settings` の `Developer settings` から同じ操作で作ります。

| 項目 | 設定値 |
|---|---|
| GitHub App name | GitHub 全体で一意な名前 |
| Homepage URL | 任意の URL（入力は必須） |
| Webhook | `Active` のチェックを外す |
| Repository permissions | 許可オーナーのワークフローが必要とする権限だけを、必要な水準で選ぶ。読むだけの権限は `Read-only` にする |
| Organization permissions | 何も選ばない |
| Account permissions | 何も選ばない |
| Enterprise permissions | 何も選ばない |
| Where can this GitHub App be installed? | `Only on this account` |

**GitHub App に付与した権限は、§5.4 の発行上限に含めると、許可オーナーのすべてのワークフローが要求できるようになります。**
付与する権限の決め方と、付与を避ける権限は、[Security Considerations](security-considerations.md) の §1.4 にあります。
Organization permissions、Account permissions、Enterprise permissions は、GATES が発行しないため付与しません。

`Create GitHub App` をクリックし、設定ページの `General` に表示される **Client ID** を控えます。
§5 で使います。

### 3.2 Installing the GitHub App

設定ページの `Install App` から、許可オーナーの `Install` をクリックします。
**`Only select repositories` を選び、トークンを発行してよいリポジトリだけを加えます。**
許可オーナーのどのワークフローも、ここで選んだインストール対象を、§3.1 で付与し §5.4 の発行上限に含めた権限で操作できます。
§9 の確認に、インストール対象のリポジトリが一つ要ります。

## 4. Getting and Verifying the Source

GATES のリポジトリを取得し、デプロイするリリースのタグをチェックアウトします。
`<バージョン>` には、`vX.Y.Z` の形のタグを入れます。
以後は、チェックアウトしたタグにある本文書に従います。

```bash
git clone https://github.com/tmknom/gates.git
cd gates
git switch --detach <バージョン>
```

続けて、同じバージョンの配布物をダウンロードし、[Release Verification](verify-release.md) の §2 と §3 の手順で検証します。
配布物は、§6.3 で関数のコードと照合するまで残しておきます。

## 5. Setting terraform.tfvars

Terraform を初期化し、例のファイルをコピーします。
初期化は、`terraform.tfvars` と Terraform の State を置く `terraform/local/` も作ります。

```bash
make -C terraform init
cp terraform/terraform.tfvars.example terraform/local/terraform.tfvars
```

`terraform/local/terraform.tfvars` に、次の値を設定します。
値が条件を満たさないと、`make tf-plan` が変数名と理由を表示して止まります。

| 変数 | 意味 | 決め方 |
|---|---|---|
| `region` | AWS リソースを作るリージョン | 既定は `ap-northeast-1`。変えるときは、Terraform のコンテナが通信してよい宛先の一覧（`scripts/terraform/proxy/allow-aws.txt`）のリージョンも変える |
| `allowed_owner_id` | 許可オーナーの不変 Owner ID | §5.1。文字列として書く |
| `allowed_owner_name` | 許可オーナーの名前 | §5.1。GitHub 上の表記のまま書く |
| `allowed_owner_type` | 許可オーナーの種別 | §5.1。`User` か `Organization` |
| `github_app_client_id` | GitHub App の Client ID | §3.1 で控えた値 |
| `gates_audience` | GATES が受け付ける Audience | §5.2 |
| `issuable_permissions` | 発行上限 | §5.4 |
| `kms_keys` | KMS Key の名前から鍵の種類への Map | §5.3 |
| `active_kms_key` | 使用中の鍵の名前 | §5.3 |
| `lambda_reserved_concurrency` | Lambda の関数の同時実行数の予約 | 既定の `-1`（予約しない）のままでよい。予約するなら、アカウントの未予約の同時実行数が 100 を下回らない値にする |
| `log_retention_in_days` | ログと監査記録の保持期間（日） | 既定は 7。監査の方針に合わせ、CloudWatch Logs が受け付ける日数（14、30、90、365 など）にする |

### 5.1 Owner ID, Name, and Type

許可オーナーの名前から、`allowed_owner_id`、`allowed_owner_name`、`allowed_owner_type` の順に値を調べます。

```bash
gh api users/<オーナー名> --jq '[.id, .login, .type] | @tsv'
```

```text
583231	octocat	User
```

### 5.2 Audience

`gates_audience` には、GATES 専用の、URL でない文字列を決めます（例：`<オーナー名>-gates`）。
ほかのサービスに向けた OIDC JWT で GATES を呼び出せないようにするためです。
`sts.amazonaws.com` と、`https://github.com` で始まる値は拒否されます。

### 5.3 KMS Keys

最初は、`key-1` という名前の KMS Key を一つだけ置き、使用中の鍵にします。

```hcl
kms_keys = {
  key-1 = "RSA_2048"
}
active_kms_key = "key-1"
```

鍵の種類は、§7 でインポートする秘密鍵の鍵長に合わせます。
秘密鍵はこの後で生成するため、GitHub が生成する鍵の鍵長として観測されている `RSA_2048` にします。
鍵長が違ったときは、§10.5 で扱います。

### 5.4 Issuable Permissions

`issuable_permissions` には、§3.1 で GitHub App に付与した Repository permissions のうち、ワークフローが要求する権限名と水準の組を、`権限名:水準` のリストで書きます。
書ける組は [Issue Token](../../.github/actions/issue-token/README.md) の「要求できる権限」にあります。
GitHub App の設定画面の表示名ではなく、そこにある権限名で書きます。

```hcl
issuable_permissions = [
  "contents:read",
  "contents:write",
  "pull_requests:read",
]
```

- **GitHub App にない権限を含めない**：含めても GitHub が拒否するため発行されないが、後で GitHub App の権限を広げたときに、`terraform.tfvars` のレビューを経ずに発行されるようになる
- **読み取りだけの要求も通すなら、`read` も書く**：`contents:write` だけを書くと、`contents:read` の要求は拒否される。GitHub App に `Read and write` で付与した権限を読み取りだけで使うワークフローがあるなら、`read` と `write` の両方を書く
- **`metadata:read` は、明示して要求するワークフローがあるときだけ書く**：GitHub が加えることがある `metadata:read` のためには要らない

書式の誤り（大文字、`:` のない要素、重複）は、`make tf-plan` が止めます。
**`members:read` のような Repository permissions でない組や、「要求できる権限」にない組は、`make tf-plan` が止めません。**
適用すると、Lambda の関数が初期化に失敗し、どの要求にもトークンを返さなくなります。
この失敗は、§9.1 のワークフローで `the Lambda function returned FunctionError` として現れます。
ログの読み方は [Troubleshooting](troubleshooting.md) の §5.3 にあります。

## 6. Plan and Apply

### 6.1 Applying

`make tf-plan` を実行します。
デプロイパッケージ（`dist/lambda.zip`）をビルドしてから Plan を作ります。

```bash
make tf-plan
```

出力で、対象の AWS アカウントとリージョンを確かめます。
Plan は 12 個のリソースの作成だけを示します。

```text
Plan: 12 to add, 0 to change, 0 to destroy.
```

問題がなければ適用し、もう一度 `make tf-plan` を実行して `No changes` を確かめます。

```bash
make tf-apply
make tf-plan
```

`make tf-apply` が失敗したら、`make tf-plan` からやり直します。

### 6.2 Checking the Created Resources

Terraform の Output を表示します。
§8 で、`github_actions_role_name` と `lambda_function_name` を使います。

```bash
make -C terraform output
```

```text
github_actions_role_name = "gates-github-actions"
kms_key_arn = "arn:aws:kms:<リージョン>:<AWS アカウント ID>:key/<鍵の ID>"
kms_key_arns = {
  "key-1" = "arn:aws:kms:<リージョン>:<AWS アカウント ID>:key/<鍵の ID>"
}
lambda_function_name = "gates"
```

KMS Key は、鍵素材を持たない `PendingImport` の状態で作られます。
§7 でインポートするまで、GATES はトークンを発行できません。

```bash
aws kms describe-key --key-id <kms_key_arn> \
  --query 'KeyMetadata.[KeyState,Origin,KeyUsage,KeySpec]' --output text
```

```text
PendingImport	EXTERNAL	SIGN_VERIFY	RSA_2048
```

### 6.3 Checking the Deployed Function Code

デプロイした関数のコードが、§4 で検証した配布物と同じであることを確かめます。
`<配布物のパス>` には、`gates-lambda-linux-arm64.zip` のパスを入れます。
2 つの出力が一致すれば、同じです。

```bash
aws lambda get-function --function-name gates --query Configuration.CodeSha256 --output text
openssl dgst -sha256 -binary <配布物のパス> | openssl base64
```

一致しないときは、§7 に進まず、[Release Verification](verify-release.md) の §4 で原因を調べます。

### 6.4 Keeping terraform/local/

`terraform/local/` は消さずに保管します。
State を失うと、Terraform は作ったリソースを管理できなくなり、以後の変更や更新ができません。

## 7. Importing the Key Material

GitHub App の設定ページの `Credentials` → `Key pairs` → `New key` で、秘密鍵をダウンロードします。
秘密鍵が手元にある時間を短くするため、すぐに次のコマンドを実行します。

```bash
make kms-import-key-material KMS_KEY_NAME=key-1 GITHUB_PRIVATE_KEY_PATH=<ダウンロードした秘密鍵のパス>
```

コマンドは、インポートの後にダウンロードした秘密鍵を削除し、結果を表示します。

```text
KMS key:     key-1 arn:aws:kms:...
Key state:   Enabled KEY_MATERIAL_DOES_NOT_EXPIRE
Fingerprint: SHA256:...
```

`Key state` が `Enabled` で、`Fingerprint` が `Key pairs` に表示された鍵のフィンガープリントと一致すれば成功です。
最後に `make tf-plan` が `No changes` を示すことを確かめます。

インポートが途中で止まったときは [Key Material Import](import-key-material.md) の §3 を、鍵長が `key_spec` と違って止まったときは §10.5 を見ます。

## 8. Registering the Connection Information

接続情報を、GitHub の Secrets と Variables に登録します。

| 名前 | 種類 | 値 |
|---|---|---|
| `GATES_AWS_ACCOUNT_ID` | Secret | GATES を適用した AWS アカウントの ID |
| `GATES_ROLE_NAME` | Variable | Output の `github_actions_role_name` |
| `GATES_FUNCTION_NAME` | Variable | Output の `lambda_function_name` |
| `GATES_AWS_REGION` | Variable | `terraform.tfvars` の `region` |
| `GATES_AUDIENCE` | Variable | `terraform.tfvars` の `gates_audience` |

AWS アカウント ID は、次のコマンドで確かめます。

```bash
aws sts get-caller-identity --query Account --output text
```

許可オーナーがユーザーアカウントなら、GATES を呼び出すリポジトリごとに登録します。
Organization なら §10.3 で、Dependabot のワークフローで使うなら §10.4 で登録します。

```bash
gh secret set GATES_AWS_ACCOUNT_ID --repo <オーナー名>/<リポジトリ名>
gh variable set GATES_ROLE_NAME --repo <オーナー名>/<リポジトリ名> --body <github_actions_role_name>
gh variable set GATES_FUNCTION_NAME --repo <オーナー名>/<リポジトリ名> --body <lambda_function_name>
gh variable set GATES_AWS_REGION --repo <オーナー名>/<リポジトリ名> --body <region>
gh variable set GATES_AUDIENCE --repo <オーナー名>/<リポジトリ名> --body <gates_audience>
```

`gh secret set` は値の入力を求めるため、AWS アカウント ID がシェルの履歴に残りません。

## 9. Checking the First Issuance

### 9.1 Running a Workflow

§8 で登録したリポジトリに、[Using GATES in Workflows](workflow-usage.md) の §1 のワークフローを置きます。
`example-owner/repo-a` は、インストール対象のリポジトリの完全名に置き換えます。
`permissions` の `contents:read` が GitHub App の権限か発行上限にないなら、両方に含まれる `read` の権限に置き換えます。
`<コミット>` には、§4 でチェックアウトしたタグのコミットの SHA（`git rev-parse HEAD` の出力）を入れます。

ワークフローを既定のブランチに置いて実行します。

```bash
gh workflow run "<ワークフローの名前>" --repo <オーナー名>/<リポジトリ名>
```

`Read repository` のステップがリポジトリの完全名を出力すれば成功です。
`Issue token` のステップのログに出る `Issued the token:` の行の `request_id` を控えます。

失敗したときは、注釈から [Troubleshooting](troubleshooting.md) で原因を引きます。
デプロイの直後に多いのは、`AssumeRoleWithWebIdentity failed`（§2.3、§2.4、§5.1 の値）と、`error code "invalid_id_token"`（`GATES_AUDIENCE` と `gates_audience` の食い違い）です。

### 9.2 Checking the Audit Record

控えた `request_id` で、監査記録を探します。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --filter-pattern '{ $.request_id = "<request_id>" }' \
  --query 'events[].message' --output json \
  | jq -c '.[] | fromjson | select(.msg == "audit record")
      | {request_id, result, owner_id: .caller.owner_id, repositories: .request.repositories}'
```

```json
{"request_id":"<request_id>","result":"success","owner_id":<Owner ID>,"repositories":["<要求したリポジトリ>"]}
```

`result` が `success` で、`owner_id` が `allowed_owner_id` なら、デプロイは完了です。

### 9.3 Next Steps

ワークフロー利用者に、接続情報の名前、インストール対象、使える権限（GitHub App に付与し、発行上限に含めた権限）、Action のコミットの SHA を伝えます。
ほかのリポジトリで GATES を使うときは、そのリポジトリにも §8 の接続情報を登録します。
デプロイの後の作業は [Operations](operations.md) にあります。

## 10. Environment-Specific Tasks

### 10.1 Creating the IAM OIDC Provider

§2.3 で Provider がなかったときは作ります。
この Provider はアカウントに一つだけ置け、ほかの用途と共有します。
作るには `iam:CreateOpenIDConnectProvider` の権限が要ります。

```bash
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com
```

Provider はあるが Client ID に `sts.amazonaws.com` がないときは、`aws iam add-client-id-to-open-id-connect-provider` で加えます。

### 10.2 Accounts that Require a Permissions Boundary

GATES の Terraform は、作る IAM Role に、自身が作る Permissions Boundary の `gates-permissions-boundary` を付けます。
IAM Role の作成に特定の Permissions Boundary を求めるアカウントでは、`arn:aws:iam::<AWS アカウント ID>:policy/gates-permissions-boundary` を許可の対象に加えてから適用します。
許可の対象に加えてから適用するまでは、別の主体が同じ名前のポリシーを任意の内容で作れます。
§6.1 の適用が `EntityAlreadyExists` で失敗したら、先に作られたポリシーと、そのポリシーを付けた Role を調べます。

### 10.3 Organization as the Allowed Owner

接続情報を、Organization 単位で一度だけ登録できます。

```bash
gh secret set GATES_AWS_ACCOUNT_ID --org <オーナー名>
gh variable set GATES_ROLE_NAME --org <オーナー名> --body <github_actions_role_name>
gh variable set GATES_FUNCTION_NAME --org <オーナー名> --body <lambda_function_name>
gh variable set GATES_AWS_REGION --org <オーナー名> --body <region>
gh variable set GATES_AUDIENCE --org <オーナー名> --body <gates_audience>
```

`--visibility` を付けないと、private リポジトリからだけ参照できます。
対象を絞るときは `--visibility selected` と `--repos` を付けます。
GitHub Free の private リポジトリと、ポリシーで Organization の値の参照を制限したリポジトリには、§8 のとおりリポジトリごとに登録します。

### 10.4 Dependabot Workflows

Dependabot が起動したワークフローは、GitHub Actions の Secrets を参照できません。
Dependabot のワークフローで GATES を使うときは、Secret を Dependabot の Secrets にも登録します。

```bash
gh secret set GATES_AWS_ACCOUNT_ID --app dependabot --repo <オーナー名>/<リポジトリ名>
```

### 10.5 When the Key Length Differs from key_spec

KMS Key の鍵の種類は後から変えられないため、`key-1` は使えません。
鍵長に合わせた `key-2` を加えて使用中の鍵にし、`key-1` は削除を予約します。
削除の予約には `kms:ScheduleKeyDeletion` の権限が要ります。
インポートが止まったときの出力で、鍵長を確かめます。
例では 4096 bit だったとします。

1. `kms_keys` に `key-2 = "RSA_4096"` を加える。`active_kms_key` は `key-1` のままにする。`make tf-plan` が `aws_kms_external_key.signing["key-2"]` の作成と Output の変更だけを示すことを確かめ、`make tf-apply` で適用する
2. GitHub App で新しい秘密鍵を生成し、`KMS_KEY_NAME=key-2` で §7 のコマンドを実行する
3. GitHub App の `Key pairs` から、最初に生成した使えない鍵を削除する
4. `active_kms_key` を `key-2` にする。`make tf-plan` が、Lambda の環境変数 `GATES_KMS_KEY_ARN`、`aws_iam_policy.lambda_sign`、Output の `kms_key_arn` の変更だけを示すことを確かめ、適用する
5. `key-1` の ARN を控えてから、`kms_keys` から `key-1` を除き、State から外す。`make tf-plan` が Output の `kms_key_arns` の変更だけを示すことを確かめ、適用する

```bash
export OLD_KMS_KEY_ARN="$(jq -r '.outputs.kms_key_arns.value["key-1"]' terraform/local/terraform.tfstate)"
make -C terraform state-rm-kms-key KMS_KEY_NAME=key-1
```

6. `key-1` の削除を予約し、状態が `PendingDeletion` であることを確かめる。7 日後に削除される

```bash
aws kms schedule-key-deletion --key-id "$OLD_KMS_KEY_ARN" --pending-window-in-days 7
aws kms describe-key --key-id "$OLD_KMS_KEY_ARN" --query 'KeyMetadata.KeyState' --output text
```

各手順の適用の後は、`make tf-plan` が `No changes` を示すことを確かめます。
手順 5 で Plan に `key-1` の削除が現れて `Instance cannot be destroyed` で止まったら、State から外していないため、適用しません。

## 11. References

- [GitHub — Registering a GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app)
- [GitHub — Installing your own GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-your-own-github-app)
- [GitHub — REST API endpoints for GitHub Actions OIDC](https://docs.github.com/en/rest/actions/oidc)
- [GitHub CLI — gh secret set](https://cli.github.com/manual/gh_secret_set)
- [GitHub CLI — gh variable set](https://cli.github.com/manual/gh_variable_set)
- [AWS — Create an OpenID Connect (OIDC) identity provider in IAM](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc.html)
