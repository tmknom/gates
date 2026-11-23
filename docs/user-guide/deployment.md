# Deployment

運用者に向けて、GATES を初めてデプロイし、最初のトークンの発行を確かめるまでの手順を示します。
上から順に進めると、許可オーナーのワークフローが Action でトークンを受け取れるようになります。
一部の環境でだけ要る作業は、末尾の §10 にあります。

## 1. Prerequisites

### 1.1 Local Tools

| ツール | 使う場面 |
|---|---|
| Git | GATES のリポジトリを取得する |
| `make` | デプロイと鍵素材のインポートを実行する |
| Docker（Docker Compose を含む） | Terraform をコンテナの中で実行する。手元の `terraform` は使わない |
| Go | Lambda のデプロイパッケージをビルドする |
| `zip`、`shasum` | デプロイパッケージを作り、確かめる |
| AWS CLI、`jq`、`xxd`、OpenSSL 3 以降 | 鍵素材をインポートし、結果を確かめる。macOS 標準の LibreSSL では動かない |
| GitHub CLI | 配布物を検証し、接続情報を登録する。`gh auth login` でログインしておく |

### 1.2 AWS Credentials

AWS の一時認証情報（`AWS_ACCESS_KEY_ID`、`AWS_SECRET_ACCESS_KEY`、`AWS_SESSION_TOKEN`）をシェルの環境変数に設定します。
`AWS_SESSION_TOKEN` のない長期のアクセスキーでは、`make tf-plan` などのコマンドが動きません。
`AWS_REGION` は、GATES をデプロイするリージョンにします。

一時認証情報の主体には、次の権限が要ります。

- IAM Role とポリシー、KMS Key と Key Policy、Lambda の関数、CloudWatch Logs のロググループの作成と変更
- KMS Key への鍵素材のインポート
- 確認に使う読み取り（`iam:GetOpenIDConnectProvider`、`kms:DescribeKey`、`lambda:GetFunction`、`logs:FilterLogEvents`）

### 1.3 IAM OIDC Provider

AWS アカウントに、GitHub Actions の OIDC JWT を信頼する IAM OIDC Provider が要ります。
GATES の Terraform は Provider を作らないため、あることを確かめます。

```bash
aws iam get-open-id-connect-provider \
  --open-id-connect-provider-arn arn:aws:iam::<AWS アカウント ID>:oidc-provider/token.actions.githubusercontent.com \
  --query ClientIDList
```

出力の配列に `sts.amazonaws.com` があれば、満たしています。
`NoSuchEntity` のエラーになったときは、§10.1 で作ります。

### 1.4 OIDC JWT of the Repositories

GATES を呼び出すリポジトリの OIDC JWT は、`sub` が不変の ID を含む形式（`repo:<オーナー名>@<Owner ID>/<リポジトリ名>@<Repository ID>:...`）でなければなりません。
名前だけの形式のリポジトリからの呼び出しは、AWS が拒否します。
新しいリポジトリは既定でこの形式ですが、古いリポジトリは名前だけの形式のことがあります。

リポジトリごとに、次のコマンドで確かめます。

```bash
gh api repos/<オーナー名>/<リポジトリ名>/actions/oidc/customization/sub
```

出力の `use_immutable_subject` が `false` で、`sub_claim_prefix` に `@` がなければ、名前だけの形式です。
その場合は §10.2 で切り替えます。

## 2. Creating and Installing the GitHub App

### 2.1 Creating the GitHub App

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

**GitHub App に付与した権限は、発行上限が許すと、保護対象でないリポジトリに対して、許可オーナーのすべてのワークフローが要求できるようになります。**
付与を避けるべき権限は、[Security Considerations](security-considerations.md) にあります。

`Create GitHub App` をクリックし、設定ページの `General` に表示される **Client ID** を控えます。

### 2.2 Installing the GitHub App

設定ページの `Install App` から、許可オーナーの `Install` をクリックします。
**`Only select repositories` を選び、許可オーナーのどのワークフローから操作されてもよいリポジトリだけを加えます。**
一部のワークフローからだけ操作させたいリポジトリは、§4.5 で保護対象にするなら加えてかまいません。
GATES は鍵素材をインポートするまで発行しないため、保護対象の一覧を適用する前に加えても、そのあいだにトークンは発行されません。

## 3. Getting and Verifying the Source

GATES のリポジトリを取得し、デプロイするリリースのタグをチェックアウトします。
`<バージョン>` には、`vX.Y.Z` の形のタグを入れます。
以後は、チェックアウトしたタグにある本文書に従います。

```bash
git clone https://github.com/tmknom/gates.git
cd gates
git switch --detach <バージョン>
```

続けて、同じバージョンの配布物をダウンロードし、[Release Verification](verify-release.md) の手順でチェックサムと Attestation を検証します。
配布物は、デプロイした関数のコードとの照合に使うため、残しておきます。

以降のコマンドは、このリポジトリのルートで、同じシェルから実行します。

## 4. Setting terraform.tfvars

Terraform を初期化し、例のファイルをコピーします。
初期化は、`terraform.tfvars` と Terraform の State を置く `terraform/local/` も作ります。

```bash
make -C terraform init
cp terraform/terraform.tfvars.example terraform/local/terraform.tfvars
```

`terraform/local/terraform.tfvars` に、以下の値を設定します。
値が条件を満たさないと、`make tf-plan` が変数名と理由を表示して止まります。

### 4.1 Allowed Owner

`allowed_owner_id`、`allowed_owner_name`、`allowed_owner_type` には、許可オーナーの不変 Owner ID、名前、種別を設定します。
次のコマンドが、この順に出力します。
`allowed_owner_id` は文字列として書きます。

```bash
gh api users/<オーナー名> --jq '[.id, .login, .type] | @tsv'
```

```text
583231	octocat	User
```

### 4.2 GitHub App and Audience

`github_app_client_id` には、GitHub App を作ったときに控えた Client ID を設定します。

`gates_audience` には、GATES 専用の、URL でない文字列を決めます（例：`<オーナー名>-gates`）。
ほかのサービスに向けた OIDC JWT で GATES を呼び出せないようにするためです。
`sts.amazonaws.com` と、`https://github.com` で始まる値は拒否されます。

### 4.3 KMS Keys

最初は、`key-1` という名前の KMS Key を一つだけ置き、使用中の鍵にします。
鍵の種類は、GitHub が生成する秘密鍵の鍵長に合わせて `RSA_2048` にします。

```hcl
kms_keys = {
  key-1 = "RSA_2048"
}
active_kms_key = "key-1"
```

### 4.4 Issuable Permissions

`issuable_permissions` には、GitHub App に付与した Repository permissions のうち、ワークフローが要求する権限名を、要求する最も高い水準とともに `権限名:水準` のリストで書きます。
権限名は、GitHub App の設定画面の表示名ではなく、[Action Reference](action-reference.md) の「要求できる権限」にある名前で書きます。

```hcl
issuable_permissions = [
  "contents:write",
  "pull_requests:read",
]
```

- **高い水準は低い水準の要求も許す**：`contents:write` を書くと、`contents:read` の要求も通る。トークンは要求した水準で発行される。読み取りにしか使わない権限は `read` で書く
- **GitHub App にない権限を含めない**：後で GitHub App の権限を広げたときに、`terraform.tfvars` のレビューを経ずに発行されるようになる

**`make tf-plan` が止めるのは書式の誤りだけです。**
「要求できる権限」にない組（`members:read` など）を含めて適用すると、Lambda の関数が初期化に失敗し、どの要求にもトークンを返さなくなります。

### 4.5 Protected Repositories

`protected_repositories` には、保護対象の一覧を YAML で書きます。
保護対象を置かないときは、例のファイルの値のままにします。
変数を消すと、`make tf-plan` が止まります。

```hcl
protected_repositories = <<-EOT
  repositories: []
EOT
```

保護対象を置くときは、例のファイルのコメントにある形で書きます。
保護対象と許可 Caller の選び方は、[Security Considerations](security-considerations.md) にあります。

```hcl
protected_repositories = <<-EOT
  repositories:
    # 共通基盤
    - id: "200"
      name: platform-infra
      callers:
        - repository_id: "100"
          repository_name: deploy-tools
          permissions:
            contents: write
            pull_requests: read
    # どの Caller にも許可しない
    - id: "201"
      name: audit-records
      callers: []
EOT
```

| キー | 書くもの |
|---|---|
| `id`、`name` | 保護対象の Repository ID と、オーナー名を含まないリポジトリ名 |
| `callers` | 保護対象の許可 Caller の列。`[]` は、どの Caller にも許可しないことを示す |
| `repository_id`、`repository_name` | 許可 Caller のリポジトリの Repository ID と、オーナー名を含まないリポジトリ名 |
| `permissions` | 許可 Caller の権限の上限。権限名をキー、許す最も高い水準を値にして、1 組以上書く。権限名と水準は「要求できる権限」にあるものにする |

Repository ID と名前は、次のコマンドがこの順に出力します。
画面の表示や記憶から書かず、出力を写します。

```bash
gh api repos/<オーナー名>/<リポジトリ名> --jq '[.id, .name] | @tsv'
```

```text
1296269	Hello-World
```

書くときは、次を守ります。

- **ID は引用符で囲む**：`id: 200` のように引用符がないと、`make tf-plan` が止まる
- **名前は、迷ったら引用符で囲む**：`123` や `true` のように、文字列以外に読める名前は、引用符がないと、適用した後に Lambda の関数が初期化に失敗する。`2001-12-14` のような日付の形の名前は、引用符がないと `make tf-plan` が止まる
- **どの Caller にも許可しないなら `callers: []` にする**：許可 Caller の `permissions` を空にすると、`make tf-plan` が止まる
- **発行上限が許さない権限は、権限の上限に書いても発行されない**：保護対象への要求は、GitHub App の権限、発行上限、権限の上限のすべてが許す必要がある
- **コメントも大きさに数える**：一覧はコメントを含めてそのまま Lambda の環境変数に入り、Plan の差分にも出る。環境変数の合計は 4096 バイトまでで、日本語は 1 文字 3 バイトになる。超えると、`make tf-plan` が `the environment variables of the Lambda function must total at most 4096 bytes as JSON` で始まるエラーで止まる

`make tf-plan` が止めるのは、YAML として読めない値、構造の誤り（キーの欠落、未知のキー、`callers` を `- ` の並びでなく `キー: 値` で書くような値の形の誤り）、ID の表記の誤り、名前と権限の文字種の誤り、ID と名前の重複、空の `permissions`、環境変数の大きさの超過です。
**次の誤りは `make tf-plan` を通り、適用すると Lambda の関数が初期化に失敗して、どの要求にもトークンを返さなくなります。**

- 権限の上限に、「要求できる権限」にない組がある（`members: read`、`contents: admin` など）
- 同じまとまり（最上位、一つの保護対象、一つの許可 Caller、一つの `permissions`）の中に同じキーが 2 つある（`name` を 2 回書くなど）
- アンカー（`&`）、エイリアス（`*`）、マージキー（`<<`）、タグ（`!!str` など）、`%TAG` の行がある
- 文字列以外に読める、引用符のない名前か権限がある（`name: 123`、`contents: true`）
- BOM か、NEL（U+0085）、LS（U+2028）、PS（U+2029）の文字がある

### 4.6 Other Variables

次の変数は、既定のままでかまいません。

- **`region`**：既定は `ap-northeast-1`。変えるときは、Terraform のコンテナが通信してよい宛先の一覧（`scripts/terraform/proxy/allow-aws.txt`）のリージョンも変える
- **`lambda_reserved_concurrency`**：既定は `-1`（予約しない）
- **`log_retention_in_days`**：ログと監査記録の保持期間。既定は 7 日。監査の方針に合わせ、CloudWatch Logs が受け付ける日数（14、30、90、365 など）にする

## 5. Plan and Apply

### 5.1 Applying

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
`make tf-apply` が失敗したら、`make tf-plan` からやり直します。

```bash
make tf-apply
make tf-plan
```

### 5.2 Checking the Output

Terraform の Output を表示します。
値は、接続情報の登録に使います。

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

KMS Key は鍵素材を持たない状態で作られるため、鍵素材をインポートするまで、GATES はトークンを発行できません。

### 5.3 Checking the Deployed Function Code

デプロイした関数のコードが、検証した配布物と同じであることを確かめます。
`<配布物のパス>` には、`gates-lambda-linux-arm64.zip` のパスを入れます。
2 つの出力が一致すれば、同じです。

```bash
aws lambda get-function --function-name gates --query Configuration.CodeSha256 --output text
openssl dgst -sha256 -binary <配布物のパス> | openssl base64
```

一致しないときは、鍵素材をインポートせず、[Release Verification](verify-release.md) で原因を調べます。

### 5.4 Keeping terraform/local/

`terraform/local/` は消さずに保管します。
State を失うと、Terraform は作ったリソースを管理できなくなり、以後の変更や更新ができません。

## 6. Importing the Key Material

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

鍵長が `key_spec` と違って止まったときは §10.6 を、ほかの理由で止まったときは [Key Material Import](import-key-material.md) を見ます。

## 7. Registering the Connection Information

接続情報を、GitHub の Secrets と Variables に登録します。

| 名前 | 種類 | 値 |
|---|---|---|
| `GATES_AWS_ACCOUNT_ID` | Secret | GATES を適用した AWS アカウントの ID（`aws sts get-caller-identity --query Account --output text`） |
| `GATES_ROLE_NAME` | Variable | Output の `github_actions_role_name` |
| `GATES_FUNCTION_NAME` | Variable | Output の `lambda_function_name` |
| `GATES_AWS_REGION` | Variable | `terraform.tfvars` の `region` |
| `GATES_AUDIENCE` | Variable | `terraform.tfvars` の `gates_audience` |

許可オーナーがユーザーアカウントなら、GATES を呼び出すリポジトリごとに登録します。
`gh secret set` は値の入力を求めるため、AWS アカウント ID がシェルの履歴に残りません。

```bash
gh secret set GATES_AWS_ACCOUNT_ID --repo <オーナー名>/<リポジトリ名>
gh variable set GATES_ROLE_NAME --repo <オーナー名>/<リポジトリ名> --body <github_actions_role_name>
gh variable set GATES_FUNCTION_NAME --repo <オーナー名>/<リポジトリ名> --body <lambda_function_name>
gh variable set GATES_AWS_REGION --repo <オーナー名>/<リポジトリ名> --body <region>
gh variable set GATES_AUDIENCE --repo <オーナー名>/<リポジトリ名> --body <gates_audience>
```

許可オーナーが Organization なら §10.4 で、Dependabot のワークフローで使うなら §10.5 で登録します。

## 8. Checking the First Issuance

### 8.1 Running a Workflow

接続情報を登録したリポジトリに、[Using GATES in Workflows](workflow-usage.md) の最小のワークフローを置きます。
次の値を置き換えます。

- **`example-owner/repo-a`**：インストール対象のうち、保護対象でないリポジトリの完全名
- **`<コミット>`**：チェックアウトしたタグのコミットの SHA（`git rev-parse HEAD` の出力）
- **`contents:read`**：GitHub App の権限か発行上限が `contents:read` を許さないなら、両方が許す `read` の権限

ワークフローを既定のブランチに置いて実行します。

```bash
gh workflow run "<ワークフローの名前>" --repo <オーナー名>/<リポジトリ名>
```

`Read repository` のステップがリポジトリの完全名を出力すれば成功です。
`Issue token` のステップのログに出る `Issued the token:` の行の `request_id` を控えます。
失敗したときは、注釈をもとに [Troubleshooting](troubleshooting.md) で原因を調べます。

### 8.2 Checking the Audit Record

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

## 9. Next Steps

ワークフロー利用者に、接続情報の名前、インストール対象、使える権限（GitHub App に付与し、発行上限が許す権限）、Action のコミットの SHA を伝えます。
保護対象を置いたなら、保護対象と、それを要求できる許可 Caller のリポジトリと権限の上限も伝えます。
ほかのリポジトリで GATES を使うときは、そのリポジトリにも接続情報を登録します。
デプロイの後の作業は [Operations](operations.md) にあります。

## 10. Environment-Specific Tasks

### 10.1 Creating the IAM OIDC Provider

Provider がなかったときは作ります。
この Provider はアカウントに一つだけ置け、ほかの用途と共有します。
作るには `iam:CreateOpenIDConnectProvider` の権限が要ります。

```bash
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com
```

Provider はあるが Client ID に `sts.amazonaws.com` がないときは、`aws iam add-client-id-to-open-id-connect-provider` で加えます。

### 10.2 Switching to the Immutable sub Format

名前だけの形式のリポジトリは、リポジトリの管理者が次のコマンドで切り替えます。
許可オーナーが Organization なら、Organization の設定（REST API の `use_immutable_subject`）でまとめて切り替えられます。

```bash
gh api --method PUT repos/<オーナー名>/<リポジトリ名>/actions/oidc/customization/sub \
  -F use_default=true -F use_immutable_subject=true
```

`sub` の形式をカスタマイズして、先頭がこの形式でなくなったリポジトリからも呼び出せません。

### 10.3 Accounts that Require a Permissions Boundary

GATES の Terraform は、作る IAM Role に、自身が作る Permissions Boundary の `gates-permissions-boundary` を付けます。
IAM Role の作成に特定の Permissions Boundary を求めるアカウントでは、`arn:aws:iam::<AWS アカウント ID>:policy/gates-permissions-boundary` を許可の対象に加えてから適用します。
許可の対象に加えてから適用するまでは、別の主体が同じ名前のポリシーを任意の内容で作れます。
適用が `EntityAlreadyExists` で失敗したら、先に作られたポリシーと、そのポリシーを付けた Role を調べます。

### 10.4 Organization as the Allowed Owner

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
GitHub Free の private リポジトリと、ポリシーで Organization の値の参照を制限したリポジトリには、リポジトリごとに登録します。

### 10.5 Dependabot Workflows

Dependabot が起動したワークフローは、GitHub Actions の Secrets を参照できません。
Dependabot のワークフローで GATES を使うときは、Secret を Dependabot の Secrets にも登録します。

```bash
gh secret set GATES_AWS_ACCOUNT_ID --app dependabot --repo <オーナー名>/<リポジトリ名>
```

### 10.6 When the Key Length Differs from key_spec

KMS Key の鍵の種類は後から変えられないため、`key-1` は使えません。
鍵長に合わせた `key-2` を加えて使用中の鍵にし、`key-1` は削除を予約します。
削除の予約には `kms:ScheduleKeyDeletion` の権限が要ります。
鍵長は、インポートが止まったときの出力で確かめます。
例では 4096 bit だったとします。

1. `kms_keys` に `key-2 = "RSA_4096"` を加える。`active_kms_key` は `key-1` のままにする。`make tf-plan` が `aws_kms_external_key.signing["key-2"]` の作成と Output の変更だけを示すことを確かめ、`make tf-apply` で適用する
2. GitHub App で新しい秘密鍵を生成し、`KMS_KEY_NAME=key-2` で鍵素材をインポートする
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

- [GATES Overview](overview.md)：GATES の仕組みと、本文書で使う用語
- [Security Considerations](security-considerations.md)：GitHub App に付与する権限、インストール対象、保護対象と許可 Caller の決め方
- [Troubleshooting](troubleshooting.md)：デプロイの直後に多い `AssumeRoleWithWebIdentity failed` と `invalid_id_token` を含む、失敗の原因と直し方
- [GitHub — Registering a GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app)
- [GitHub — Installing your own GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-your-own-github-app)
- [GitHub — REST API endpoints for GitHub Actions OIDC](https://docs.github.com/en/rest/actions/oidc)
- [GitHub CLI — gh secret set](https://cli.github.com/manual/gh_secret_set)
- [GitHub CLI — gh variable set](https://cli.github.com/manual/gh_variable_set)
- [AWS — Create an OpenID Connect (OIDC) identity provider in IAM](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc.html)
