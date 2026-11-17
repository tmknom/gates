# Deployment

本ドキュメントは、**GATES 運用者**（GATES を AWS にデプロイして運用する人。以下、運用者）が、GATES を初めてデプロイし、最初のトークンの発行を確かめるまでのチュートリアルです。
上から順に進めると、**許可オーナー**（GATES がトークンを発行してよいと運用者が設定した GitHub のユーザーアカウントか Organization）のワークフローが、GATES の Docker Action（以下、Action）で、指定したリポジトリを読むトークンを受け取れるようになります。

GATES の仕組みと、用意するもの、費用が発生するリソースは [GATES Overview](overview.md) にあります。

## 1. Order of the Work

作業は次の順に進めます。

| 節 | 作業 | 場所 |
|---|---|---|
| §2 | 前提を確かめる | 手元、AWS、GitHub |
| §3 | GitHub App を作成し、許可オーナーにインストールする | GitHub |
| §4 | GATES のソースを取得し、配布物を検証する | 手元 |
| §5 | `terraform.tfvars` を設定する | 手元 |
| §6 | Terraform で AWS リソースを適用し、デプロイしたコードを照合する | 手元、AWS |
| §7 | GitHub App Private Key を生成し、KMS Key にインポートする | GitHub、手元、AWS |
| §8 | 接続情報を GitHub の Secrets と Variables に登録する | GitHub |
| §9 | 最初の発行を確かめる | GitHub、AWS |

GitHub App Private Key（以下、秘密鍵）は、§6 で AWS リソースを適用した後の §7 で生成します。
秘密鍵を取り込む先の KMS Key は §6 で作られるため、先に秘密鍵を生成すると、取り込むまでのあいだ秘密鍵が手元に残り続けるからです。

§4 以降のコマンドは、§4 で取得した GATES のリポジトリのルートで、同じシェルから実行します。

## 2. Prerequisites

### 2.1 Local Tools

手元に次のツールを用意します。

| ツール | 使う場面 |
|---|---|
| Git | GATES のリポジトリを取得する |
| `make` | デプロイと **鍵素材**（KMS Key にインポートする秘密鍵の中身）のインポートの各操作を実行する |
| Docker（Docker Compose を含む） | Terraform をコンテナの中で実行する。手元の `terraform` コマンドは使わない |
| Go | Lambda のデプロイパッケージを手元でビルドする |
| `zip`、`shasum` | デプロイパッケージを作り、Plan と適用のあいだでパッケージが変わっていないことを確かめる |
| AWS CLI、`jq`、`xxd`、OpenSSL 3 以降 | 鍵素材を KMS にインポートし、デプロイの結果を確かめる。macOS に標準で入っている LibreSSL ではインポートが動かない |
| GitHub CLI | 配布物を検証し、接続情報（Action が GATES を呼び出すのに使う値。§8）を登録する。`gh auth login` でログインしておく |

### 2.2 AWS Credentials

AWS の一時認証情報（`AWS_ACCESS_KEY_ID`、`AWS_SECRET_ACCESS_KEY`、`AWS_SESSION_TOKEN`）を、シェルの環境変数に設定します。
`make tf-plan`、`make tf-apply`、`make kms-import-key-material` は、`AWS_SESSION_TOKEN` のない長期のアクセスキーでは起動しません。
長期のアクセスキーを手元に置かずに済むようにするためです。

一時認証情報の主体には、次の権限が要ります。

- IAM Role と IAM ポリシー、KMS Key と Key Policy、Lambda の関数、CloudWatch Logs のロググループを作成し、変更する権限（Terraform の適用）
- KMS Key に鍵素材をインポートする権限
- 前提と結果の確認に使う読み取りの権限（`iam:GetOpenIDConnectProvider`、`lambda:GetAccountSettings`、`kms:DescribeKey`、`lambda:GetFunction`、`logs:FilterLogEvents`）
- IAM OIDC Provider を作るか変えるとき（§2.3）は、その権限（`iam:CreateOpenIDConnectProvider`、`iam:AddClientIDToOpenIDConnectProvider`）
- 鍵長が違ってインポートが止まったとき（§7.2）は、KMS Key の削除を予約する権限（`kms:ScheduleKeyDeletion`）

AWS CLI のリージョン（`AWS_REGION`）は、§5 で `region` に設定するリージョンにします。

### 2.3 IAM OIDC Provider

AWS アカウントに、`https://token.actions.githubusercontent.com` を指す **IAM OIDC Provider**（AWS STS が GitHub Actions の OIDC JWT を信頼するための設定）が要ります。
**OIDC JWT** は、ワークフローがどのオーナーのどのリポジトリで動いているかを GitHub が署名して示すトークンで、Action はこれを示して AWS の IAM Role を引き受け、GATES を呼び出します。
この URL を指す Provider はアカウントに一つしか置けず、ほかの用途と共有するため、GATES の Terraform は Provider を作らず、アカウントにあるものを参照します。
Provider がない間は、`make tf-plan` が失敗します。

Provider があり、その Client ID に `sts.amazonaws.com` が含まれることを確かめます。
`<AWS アカウント ID>` には、GATES を適用するアカウントの ID を入れます。

```bash
aws iam get-open-id-connect-provider \
  --open-id-connect-provider-arn arn:aws:iam::<AWS アカウント ID>:oidc-provider/token.actions.githubusercontent.com \
  --query ClientIDList
```

出力の配列に `sts.amazonaws.com` があれば、満たしています。
Provider がなく `NoSuchEntity` のエラーになったときは、作成します。

```bash
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com
```

Provider はあるが Client ID に `sts.amazonaws.com` がないときは、`aws iam add-client-id-to-open-id-connect-provider` で加えます。
GATES の検証環境では Provider を別の手段で作っているため、これらの変更のコマンドは実行していません。

### 2.4 Permissions Boundary

GATES の Terraform は、作成する 2 つの IAM Role に、自身が作る **Permissions Boundary**（IAM Role が持てる権限の上限を定める IAM ポリシー）の `gates-permissions-boundary` を付けます。
IAM Role の作成に特定の Permissions Boundary を求めるアカウントでは、`gates-permissions-boundary` の ARN（`arn:aws:iam::<AWS アカウント ID>:policy/gates-permissions-boundary`）を、アカウントの側で許可の対象に加えてから適用します。
求めないアカウントでは、何もしません。

許可の対象に加えてから GATES を適用するまでは、別の主体が同じ名前のポリシーを任意の内容で作れます。
§6 の適用が `EntityAlreadyExists` で失敗したら、そのポリシーと、それを付けた IAM Role を調べます。

### 2.5 OIDC JWT of the Allowed Owner's Repositories

GATES を呼び出すリポジトリの OIDC JWT は、呼び出し元を示す `sub` が、`repo:<オーナー名>@<Owner ID>/<リポジトリ名>@<Repository ID>:...` のように、名前に加えて変わらない ID を含む形式でなければなりません。
IAM Role の引き受けの条件（Trust Policy）が、この形式で許可オーナーを ID まで含めて照合し、名前だけの形式を拒否するためです（[Requirements](../spec/requirements.md) の FR-AUTHN-005）。

2026 年 7 月 15 日より後に作られたリポジトリは、既定でこの形式を使います。
それより前に作られたリポジトリは、Organization かリポジトリの設定で切り替えるまで、名前だけの形式を使います。
同日より後に改名や移管をしたリポジトリも、ID を含む形式に移ります。
リポジトリの現在の形式は、次のコマンドで確かめます。

```bash
gh api repos/<オーナー名>/<リポジトリ名>/actions/oidc/customization/sub
```

名前だけの形式のリポジトリでは、`sub_claim_prefix` に `@` がなく、`use_immutable_subject` が `false` です。

```json
{"use_default":true,"use_immutable_subject":false,"sub_claim_prefix":"repo:octocat/example-legacy-repo"}
```

切り替えるときは、リポジトリごとに次のコマンドを実行します。

```bash
gh api --method PUT repos/<オーナー名>/<リポジトリ名>/actions/oidc/customization/sub \
  -F use_default=true -F use_immutable_subject=true
```

許可オーナーが Organization なら、Organization の設定（REST API の `use_immutable_subject`）でまとめて切り替えられます。
GATES の検証環境ではリポジトリが既定でこの形式だったため、切り替えのコマンドは実行していません。

名前だけの形式のまま GATES を呼び出すと、AWS STS が IAM Role の引き受けを拒否し（[AWS STS Legacy Subject](../research/aws/sts-legacy-subject.md)）、Action は `AssumeRoleWithWebIdentity failed` の注釈で失敗します。
`sub` の形式をカスタマイズして、先頭がこの形式でなくなったリポジトリも同じく失敗します。

## 3. Creating and Installing the GitHub App

### 3.1 Creating the GitHub App

GATES は、運用者が作る GitHub App としてトークンを発行します。
許可オーナーがユーザーアカウントなら、そのアカウントの `Settings` → `Developer settings` → `GitHub Apps` → `New GitHub App` の順に開き、次のように設定します。
許可オーナーが Organization なら、Organization の `Settings` の `Developer settings` から同じ操作で作ります。

| 項目 | 設定値 |
|---|---|
| GitHub App name | 運用者が決める。GitHub 全体で一意でなければならない |
| Homepage URL | 運用者が決める（入力は必須） |
| Webhook | `Active` のチェックを外す。GATES は Webhook を受け取らない |
| Repository permissions | `Contents` を `Read-only` にする |
| Account permissions | 何も選択しない |
| Where can this GitHub App be installed? | `Only on this account` を選ぶ |

最後に `Create GitHub App` をクリックすると、GitHub App が作成されます。
GATES の検証環境では、`Contents` を `Read-only` にすると、`Metadata` も自動で `Read-only` になりました。

権限を `Contents` の `Read-only` にとどめるのは、GATES を経ずに GitHub App として署名された場合に、得られるトークンを読み取りの権限に限るためです（[Security Considerations](security-considerations.md) の §2）。
GATES が発行するトークンの権限は、GitHub App の権限によらず、`contents:read` と、GitHub が加えることがあるリポジトリの基本情報を読む `metadata:read` に限られます。

作成した GitHub App の設定ページ（`General`）に表示される **Client ID** を控えます。
§5 で `github_app_client_id` に設定します。

### 3.2 Installing the GitHub App

GitHub App の設定ページの左のメニューから `Install App` を開き、許可オーナーの `Install` をクリックします。
インストール先は `Only select repositories` を選び、`Select repositories` でトークンを発行してよいリポジトリだけを加えて、`Install` をクリックします。

ここで選ぶリポジトリ（**インストール対象**）が、許可オーナーのどのワークフローからでも読める範囲になります。
GATES は呼び出し元を許可オーナー単位でしか区別せず、許可オーナーのワークフローであれば、インストール対象のどのリポジトリのトークンも受け取れるためです。
`All repositories` を選ぶと、許可オーナーのすべてのリポジトリがこの範囲に入ります。
インストール対象に含めてよいリポジトリの考え方は、[Security Considerations](security-considerations.md) の §2 にあります。

インストール対象は、後から GATES の設定を変えずに変更できます。
§9 の確認には、インストール対象のリポジトリが一つ要ります。

## 4. Getting and Verifying the Source

GATES のリポジトリを取得し、デプロイするリリースのタグをチェックアウトします。
`<バージョン>` には、`vX.Y.Z` の形のリリースのタグを入れます。
以後の手順は、チェックアウトしたタグにある本ドキュメントに従います。

```bash
git clone https://github.com/tmknom/gates.git
cd gates
git switch --detach <バージョン>
```

続けて、同じバージョンの配布物（Lambda の関数のコードの zip）をダウンロードし、[Release Verification](verify-release.md) の §3 と §4 の手順で、チェックサムと Attestation を検証します。
GATES の Terraform が Lambda に上げるのは配布物ではなく、手元のソースから `make tf-plan` がビルドする zip（**デプロイパッケージ**）です。
デプロイパッケージは、リリースのタグからビルドすると配布物と同じバイト列になるように作られているため、§6 の最後に、デプロイした関数のコードを検証した配布物と照合します。
配布物は、照合が終わるまで残しておきます。

## 5. Setting terraform.tfvars

最初に、Terraform を初期化します。
初期化は Terraform のコンテナの中で、イメージに入れた Provider を使って行い、AWS の認証情報は使いません。
初期化は、`terraform.tfvars` や State を置く `terraform/local/` も作ります。

```bash
make -C terraform init
```

出力の最後に `Terraform has been successfully initialized!` が出れば、成功しています。
続けて、例のファイルを `terraform/local/terraform.tfvars` にコピーします。

```bash
cp terraform/terraform.tfvars.example terraform/local/terraform.tfvars
```

`terraform/local/` には、`terraform.tfvars` のほかに、Terraform の State と Plan が置かれます。

### 5.1 Variables

`terraform/local/terraform.tfvars` に、次の値を設定します。

| 変数 | 意味 | 決め方 |
|---|---|---|
| `region` | GATES の AWS リソースを作るリージョン | 既定は `ap-northeast-1`。変えるときは、Terraform のコンテナが通信してよい宛先の一覧（`scripts/terraform/proxy/allow-aws.txt`）のリージョンも変える |
| `allowed_owner_id` | 許可オーナーの **不変 Owner ID**（GitHub がアカウントに割り当てる、名前を変えても変わらない数値の ID） | §5.2 のコマンドで調べる。文字列として書く |
| `allowed_owner_name` | 許可オーナーの名前 | §5.2 のコマンドで調べる。GitHub 上の表記のまま、大文字と小文字も同じに書く |
| `allowed_owner_type` | 許可オーナーの種別 | `User` か `Organization`。§5.2 のコマンドで調べる |
| `github_app_client_id` | GitHub App の Client ID | §3.1 で控えた値 |
| `gates_audience` | GATES 用の OIDC JWT の Audience（OIDC JWT の宛先を示す値） | §5.3 |
| `kms_keys` | KMS Key の名前から鍵の種類（`key_spec`）への Map | §5.4 |
| `active_kms_key` | Lambda が署名に使う KMS Key（**使用中の鍵**）の、`kms_keys` での名前 | §5.4 |
| `lambda_reserved_concurrency` | Lambda の関数の同時実行数の予約 | §5.5 |
| `log_retention_in_days` | Lambda のログと監査記録の保持期間（日） | §5.6 |

`lambda_package_path` は `terraform.tfvars` に書きません。
`make tf-plan` がデプロイパッケージを作り、そのパスを渡します。

値が変数の検証の条件を満たさないと、`make tf-plan` が変数名とエラーの文言を表示して止まります。

### 5.2 Owner ID, Name, and Type

許可オーナーの名前から、不変 Owner ID、GitHub 上の表記の名前、種別を調べます。
Organization の名前でも同じコマンドで調べられます。

```bash
gh api users/<オーナー名> --jq '[.id, .login, .type] | @tsv'
```

出力は、`allowed_owner_id`、`allowed_owner_name`、`allowed_owner_type` の順に対応します。

```text
583231	octocat	User
```

許可オーナーの名前は、IAM Role の Trust Policy と Lambda の設定の両方に入ります。
許可オーナーの名前が変わったら、`allowed_owner_name` を新しい名前にして適用し直すまで、発行は止まります。

### 5.3 Audience

`gates_audience` は、GATES の Lambda に送る OIDC JWT の Audience です。
GATES は、OIDC JWT の Audience がこの値と完全に一致するときだけ受け付けます。
ほかのサービスに向けた OIDC JWT で GATES を呼び出せないよう、GATES 専用の、URL でない文字列にします（例：`<オーナー名>-gates`）。

次の値は、変数の検証で拒否されます。

- `sts.amazonaws.com`：Action が AWS STS に送る OIDC JWT の Audience と同じ値
- `https://github.com` と、`https://github.com/` で始まる値（大文字と小文字を区別しない）：GitHub は、`https://github.com` や、呼び出し元のオーナーやリポジトリの URL を Audience にした OIDC JWT も発行し、オーナーの URL は Audience を指定しない OIDC JWT にも入るため

### 5.4 KMS Keys

最初は、`key-1` という名前の KMS Key を一つだけ置き、それを使用中の鍵にします。
鍵の名前は `key-1`、`key-2` のように番号で付け、後の鍵の更新では使ったことのない次の番号を使います。

```hcl
kms_keys = {
  key-1 = "RSA_2048"
}
active_kms_key = "key-1"
```

`key_spec` は、§7 でインポートする秘密鍵の鍵長に合わせ、`RSA_2048`、`RSA_3072`、`RSA_4096` のどれかにします。
秘密鍵は適用の後に生成するため、ここでは鍵長の見込みで決めます。
GitHub は生成する鍵の鍵長を文書で約束していませんが、GATES の検証環境で生成された鍵は 2048 bit だったため、`RSA_2048` を見込みとします。
見込みが外れたときの扱いは §7.2 にあります。

### 5.5 Reserved Concurrency

`lambda_reserved_concurrency` は、GATES の Lambda の関数のために予約する同時実行数で、関数が同時に動ける数の上限にもなります。
既定の `-1` では予約しません。

予約するときは、AWS アカウントの未予約の同時実行数から予約する数を引いても、Lambda が求める未予約の下限（100）を下回らない値にします。
下回る値では適用が失敗します。
アカウントの未予約の同時実行数は、次のコマンドで確かめます。

```bash
aws lambda get-account-settings --query AccountLimit.UnreservedConcurrentExecutions
```

`0` を設定すると、GATES の Lambda のすべての呼び出しが拒否されます。

### 5.6 Log Retention

`log_retention_in_days` は、Lambda のログの保持期間で、**監査記録**（GATES の Lambda が要求ごとに、呼び出し元、要求、結果を、トークンを含めずに CloudWatch Logs に残す 1 行の JSON）の保持期間でもあります。
既定は 7 日です。
監査記録に求める保持期間は組織の方針によって異なるため、その方針に合わせて決めます。
値は、CloudWatch Logs が受け付ける日数（1、3、5、7、14、30、60、90、120、150、180、365、400、545、731、1096、1827、2192、2557、2922、3288、3653）のどれかにします。

## 6. Plan and Apply

### 6.1 Plan

`make tf-plan` を実行します。
`make tf-plan` は、デプロイパッケージ（`dist/lambda.zip`）をビルドしてから Terraform の Plan を作り、`terraform/local/plan.tfplan` に保存します。
デプロイパッケージは、`go.mod` の `toolchain` の行が指すバージョンの Go でビルドします。
手元の Go がそのバージョンでなければ、Go のコマンドがそのバージョンを取得して使います。

```bash
make tf-plan
```

出力で、対象の AWS アカウントとリージョン、変更の内容を確かめます。
初めての適用で `kms_keys` が一つなら、Plan は 12 個のリソースの作成だけを示し、変更と削除を含みません。

```text
Plan: 12 to add, 0 to change, 0 to destroy.
```

作成するのは、Lambda の関数とそのロググループ、Lambda の **実行 Role**（Lambda が KMS での署名とログの書き込みに使う IAM Role）、GitHub Actions のワークフローが引き受ける IAM Role、それぞれの IAM ポリシーと、それを IAM Role に付ける設定、Permissions Boundary、KMS Key です。
AWS リソースの構成は [AWS Infrastructure Design](../design/aws-infrastructure.md) の §2 にあります。

### 6.2 Apply

Plan の内容に問題がなければ、`make tf-apply` で保存した Plan を適用します。
`make tf-apply` は、デプロイパッケージが Plan のときと同じであることを確かめてから適用します。

```bash
make tf-apply
```

適用の後に、もう一度 `make tf-plan` を実行し、`No changes` を示すことを確かめます。

```bash
make tf-plan
```

`make tf-apply` が失敗したら、`make tf-plan` から作り直し、Plan を確かめ直してから適用します。
何も適用せずに失敗した場合でも、Terraform が State を書き直すため、保存した Plan が古くなり、次の適用が `Saved plan is stale` で止まることがあります。

### 6.3 Checking the Created Resources

Terraform の **Output**（State に記録した、ほかの手順で使う値）を表示します。

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

`kms_key_arn` は使用中の鍵の ARN、`kms_key_arns` は鍵の名前ごとの ARN、`github_actions_role_name` と `lambda_function_name` は §8 で登録する IAM Role の名前と関数名です。

KMS Key は、鍵素材を持たない `PendingImport` の状態で作られます。
`kms_key_arn` の値を指定して状態を確かめます。

```bash
aws kms describe-key --key-id <kms_key_arn> \
  --query 'KeyMetadata.[KeyState,Origin,KeyUsage,KeySpec]' --output text
```

```text
PendingImport	EXTERNAL	SIGN_VERIFY	RSA_2048
```

`KeySpec` は、`kms_keys` の `key-1` の値です。
この時点では KMS Key で署名できないため、GATES はまだトークンを発行できません。

### 6.4 Checking the Deployed Function Code

デプロイした関数のコードが、§4 で検証した配布物と同じであることを確かめます。
次のコマンドの出力が一致すれば、関数のコードは配布物と同じです。
`<配布物のパス>` には、§4 でダウンロードした `gates-lambda-linux-arm64.zip` のパスを入れます。

```bash
aws lambda get-function --function-name gates --query Configuration.CodeSha256 --output text
openssl dgst -sha256 -binary <配布物のパス> | openssl base64
```

一致しないときは、§7 に進まず、[Release Verification](verify-release.md) の §5 の手順で原因を切り分けます。

### 6.5 Keeping terraform/local/

`terraform/local/` は、消さずに保管します。
State を失うと、Terraform は作成済みのリソースを管理できなくなります。
その状態で適用し直すと、同じ名前の IAM ポリシーや IAM Role の作成が `EntityAlreadyExists` で失敗します。
State には秘密鍵は入りませんが、IAM と Lambda の構成が入るため、このディレクトリは Git の管理外にしてあります。

## 7. Importing the Key Material

### 7.1 Running the Import

[Key Material Import](import-key-material.md) の手順で、GitHub App で秘密鍵を生成し、その鍵素材を `key-1` の KMS Key にインポートします。
鍵素材を取り込むと、GATES の Lambda は KMS Key で GitHub App JWT に署名できるようになります。
秘密鍵が手元にある時間を短くするため、秘密鍵をダウンロードしたら、すぐに次のコマンドを実行します。

```bash
make kms-import-key-material KMS_KEY_NAME=key-1 GITHUB_PRIVATE_KEY_PATH=<ダウンロードした秘密鍵のパス>
```

インポートが終わると、ダウンロードした秘密鍵は削除され、KMS Key の状態と公開鍵のフィンガープリントが表示されます。

```text
KMS key:     key-1 arn:aws:kms:...
Key state:   Enabled KEY_MATERIAL_DOES_NOT_EXPIRE
Fingerprint: SHA256:...
```

`Key state` が `Enabled` で、`Fingerprint` の値が GitHub App の `Credentials` → `Key pairs` に表示された鍵のフィンガープリントと一致すれば、成功しています。
インポートが途中で止まったときの扱いは、[Key Material Import](import-key-material.md) の §4 にあります。
鍵長が `key_spec` と違って止まったときは、§7.2 に従います。

インポートの後に `make tf-plan` を実行し、`No changes` を示すことを確かめます。
KMS Key の状態が `PendingImport` から `Enabled` に変わっても、Terraform は鍵の状態を設定に持たないため、Plan に差分は出ません。

### 7.2 When the Key Length Differs from key_spec

秘密鍵の鍵長が `key_spec` と違うと、`make kms-import-key-material` はインポートの前に止まり、ダウンロードした秘密鍵を削除します。
KMS Key の `key_spec` は作成の後に変えられないため、`key-1` は使えません。
鍵長に合わせた `key_spec` の `key-2` を加えて使用中の鍵にし、`key-1` を Terraform の管理から外して削除を予約します。

止まったときの出力で、秘密鍵の鍵長を確かめます。
例では、鍵長が 4096 bit だったとします。

まず、`kms_keys` に `key-2` を加えます。
`active_kms_key` は `key-1` のままにします。

```hcl
kms_keys = {
  key-1 = "RSA_2048"
  key-2 = "RSA_4096"
}
active_kms_key = "key-1"
```

`make tf-plan` が `aws_kms_external_key.signing["key-2"]` の作成と、Output の `kms_key_arns` の変更だけを示すことを確かめてから適用し、適用の後の `make tf-plan` が `No changes` を示すことを確かめます。

```bash
make tf-plan
make tf-apply
make tf-plan
```

GitHub App で新しい秘密鍵を生成し、`key-2` にインポートします。
フィンガープリントの照合は §7.1 と同じです。

```bash
make kms-import-key-material KMS_KEY_NAME=key-2 GITHUB_PRIVATE_KEY_PATH=<ダウンロードした秘密鍵のパス>
```

インポートが成功したら、GitHub App の `Key pairs` から、最初に生成した使えない鍵を削除します。
GitHub は GitHub App の最後の鍵を削除させないため、この削除は新しい鍵を生成した後に行います。
`Key pairs` に残った鍵のフィンガープリントが、`key-2` のインポートで表示された `Fingerprint` と一致することを確かめます。

続けて、使用中の鍵を `key-2` に切り替えます。

```hcl
active_kms_key = "key-2"
```

`make tf-plan` が次の変更だけを示すことを確かめてから適用し、適用の後の `make tf-plan` が `No changes` を示すことを確かめます。

- `aws_lambda_function.gates` の環境変数 `GATES_KMS_KEY_ARN` が `key-2` の ARN に変わる
- `aws_iam_policy.lambda_sign` の `Resource` が `key-2` の ARN に変わる
- Output の `kms_key_arn` が `key-2` の ARN に変わる

```bash
make tf-plan
make tf-apply
make tf-plan
```

最後に、`key-1` を退役させます。
`key-1` の ARN を控えてから、`kms_keys` から `key-1` を除きます。

```bash
export OLD_KMS_KEY_ARN="$(jq -r '.outputs.kms_key_arns.value["key-1"]' terraform/local/terraform.tfstate)"
```

```hcl
kms_keys = {
  key-2 = "RSA_4096"
}
active_kms_key = "key-2"
```

KMS Key には Terraform による削除を止める設定（`prevent_destroy`）を付けてあり、`kms_keys` から除いた KMS Key の削除も拒否されるため、Terraform には削除させません。
`key-1` を、削除せずに State から外します。
外す前の State は `terraform/local/terraform.tfstate.key-1.backup` に残ります。

```bash
make -C terraform state-rm-kms-key KMS_KEY_NAME=key-1
```

`make tf-plan` が、リソースの変更を示さず、Output の `kms_key_arns` から `key-1` が消える変更だけを示すことを確かめてから適用し、適用の後の `make tf-plan` が `No changes` を示すことを確かめます。
`key-1` の削除が現れて `Instance cannot be destroyed` で止まったら、State から外していません。
`key-1` の作成が現れたら、`kms_keys` から除いていません。
どちらの場合も適用しません。

```bash
make tf-plan
make tf-apply
make tf-plan
```

`key-1` の削除を予約し、状態が `PendingDeletion` であることを確かめます。
`key-1` は鍵素材を持たず署名に使えないため、削除の前に無効にする必要はありません。

```bash
aws kms schedule-key-deletion --key-id "$OLD_KMS_KEY_ARN" --pending-window-in-days 7
aws kms describe-key --key-id "$OLD_KMS_KEY_ARN" --query 'KeyMetadata.KeyState' --output text
```

予約期間の 7 日を過ぎると、KMS は `key-1` を削除します。

この手順は、鍵の更新で検証環境で確かめた操作（KMS Key の追加、使用中の鍵の切り替え、State からの除外と削除の予約）を組み合わせたものです。
初めてのデプロイで鍵長が違った場合を通しては確かめていません。

## 8. Registering the Connection Information

GATES を呼び出すワークフローのために、**接続情報**（Action が GATES を呼び出すのに使う、IAM Role、関数名、リージョン、Audience の値）を GitHub の Secrets と Variables に登録します。
ワークフローは値を直接書かずにこれらを参照するため、関数名などを変えたときも、登録した値の更新だけで済みます。

| 名前 | 種類 | 値 |
|---|---|---|
| `GATES_AWS_ACCOUNT_ID` | Secret | GATES を適用した AWS アカウントの ID |
| `GATES_ROLE_NAME` | Variable | Output の `github_actions_role_name` |
| `GATES_FUNCTION_NAME` | Variable | Output の `lambda_function_name` |
| `GATES_AWS_REGION` | Variable | `terraform.tfvars` の `region` |
| `GATES_AUDIENCE` | Variable | `terraform.tfvars` の `gates_audience` |

AWS アカウント ID は、公開しない値として Secret に登録します。
Secret の値は、ワークフローのログでマスクされます。
AWS アカウント ID は、適用に使った認証情報で次のコマンドを実行すると確かめられます。

```bash
aws sts get-caller-identity --query Account --output text
```

登録する単位は、許可オーナーの種別で決まります。
許可オーナーがユーザーアカウントなら、GATES を呼び出すリポジトリごとに登録します。
ユーザーアカウントには、アカウント全体で共有する Secrets と Variables がないためです。

```bash
gh secret set GATES_AWS_ACCOUNT_ID --repo <オーナー名>/<リポジトリ名>
gh variable set GATES_ROLE_NAME --repo <オーナー名>/<リポジトリ名> --body <github_actions_role_name>
gh variable set GATES_FUNCTION_NAME --repo <オーナー名>/<リポジトリ名> --body <lambda_function_name>
gh variable set GATES_AWS_REGION --repo <オーナー名>/<リポジトリ名> --body <region>
gh variable set GATES_AUDIENCE --repo <オーナー名>/<リポジトリ名> --body <gates_audience>
```

`gh secret set` は `--body` を付けないと値の入力を求めるため、AWS アカウント ID をコマンドラインとシェルの履歴に残さずに済みます。

許可オーナーが Organization なら、Organization 単位に一度だけ登録できます。
`--visibility` を付けないと、登録した値は private リポジトリからだけ参照できます。
参照できるリポジトリを絞るときは、`--visibility selected` と `--repos` で対象を指定します。
GitHub Free の private リポジトリと、ポリシーで Organization の値の参照を制限したリポジトリからは、Organization 単位の値を参照できないため、リポジトリごとに登録します。
Organization を許可オーナーにした構成は、GATES の検証環境では確かめていません。

```bash
gh secret set GATES_AWS_ACCOUNT_ID --org <オーナー名>
gh variable set GATES_ROLE_NAME --org <オーナー名> --body <github_actions_role_name>
gh variable set GATES_FUNCTION_NAME --org <オーナー名> --body <lambda_function_name>
gh variable set GATES_AWS_REGION --org <オーナー名> --body <region>
gh variable set GATES_AUDIENCE --org <オーナー名> --body <gates_audience>
```

Dependabot が起動したワークフローは、GitHub Actions の Secrets を参照できません。
Dependabot のワークフローで GATES を使うときは、`GATES_AWS_ACCOUNT_ID` を Dependabot の Secrets にも登録します。
Variables は Dependabot のワークフローからも参照できるため、登録し直す必要はありません。

```bash
gh secret set GATES_AWS_ACCOUNT_ID --app dependabot --repo <オーナー名>/<リポジトリ名>
```

## 9. Checking the First Issuance

### 9.1 Running a Workflow

§8 で接続情報を登録した許可オーナーのリポジトリに、[Using GATES in Workflows](workflow-usage.md) の §2 の最小のワークフローを置きます。
`repositories` と `Read repository` のステップの `example-owner/repo-a` は、§3.2 でインストール対象に加えたリポジトリの完全名（`<オーナー名>/<リポジトリ名>`）に置き換えます。
`<コミット>` には、§4 でチェックアウトしたタグのコミットの SHA（`git rev-parse HEAD` の出力）を入れます。
このコミットは、ワークフロー利用者に Action の参照先として伝えるものでもあります。

ワークフローを既定のブランチに置いて、実行します。
`<ワークフローの名前>` は、ワークフローの `name` の値です。

```bash
gh workflow run "<ワークフローの名前>" --repo <オーナー名>/<リポジトリ名>
```

ワークフローの実行が成功し、`Read repository` のステップが読むリポジトリの完全名を出力すれば、トークンでそのリポジトリを読めています。
`Issue token` のステップのログに出る `Issued the token:` の行の `request_id` を控えます。
§9.2 で監査記録を探すのに使います。

失敗したときは、`Issue token` のステップの `::error::` の注釈の先頭の文言で、失敗した段階を見分けられます。
段階ごとの見直す箇所は、[Issue Token](../../.github/actions/issue-token/README.md) の「失敗の読み方」にあります。
デプロイの直後に起きやすいのは、次の失敗です。

| 注釈の先頭 | 確かめること |
|---|---|
| `ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set` | Action を置いたジョブの `permissions` に `id-token: write` がある。ジョブに `permissions` がなければ、ワークフローの `permissions` にある |
| `AssumeRoleWithWebIdentity failed` | IAM OIDC Provider がある（§2.3）。リポジトリの `sub` が ID を含む形式である（§2.5）。`GATES_AWS_ACCOUNT_ID`、`GATES_ROLE_NAME`、`GATES_AWS_REGION` が §8 の値である。`allowed_owner_id` と `allowed_owner_name` が許可オーナーの値である |
| `the Lambda Invoke failed` | `GATES_FUNCTION_NAME` と `GATES_AWS_REGION` が §8 の値である |
| `GATES returned a failure response` | 注釈の `error.code` によって次を確かめる。`invalid_id_token` なら、`GATES_AUDIENCE` が `gates_audience` と一致する。`authorization_denied` なら、読むリポジトリが許可オーナーのものである。`installation_unavailable` なら、GitHub App が許可オーナーにインストールされていて、`github_app_client_id` がその GitHub App の Client ID である。`github_issuance_rejected` なら、読むリポジトリがインストール対象に含まれる。`internal_error` なら、使用中の鍵に鍵素材をインポートしてある（§7） |

ほかの失敗と、エラーコードごとの原因と直し方は、[Troubleshooting](troubleshooting.md) にあります。

### 9.2 Checking the Audit Record

§9.1 の `request_id` で、その実行の監査記録を探し、主な項目を取り出します。
監査記録は、ロググループ `/aws/lambda/gates` の、`msg` が `audit record` の行です。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --filter-pattern '{ $.request_id = "<request_id>" }' \
  --query 'events[].message' --output json \
  | jq -c '.[] | fromjson | select(.msg == "audit record")
      | {request_id, result, owner_id: .caller.owner_id, run_id: .caller.run_id, repositories: .request.repositories}'
```

```json
{"request_id":"<request_id>","result":"success","owner_id":<Owner ID>,"run_id":<ワークフロー実行の ID>,"repositories":["<読むリポジトリ>"]}
```

`result` が `success` で、`owner_id` が `allowed_owner_id`、`run_id` が §9.1 のワークフロー実行の ID、`repositories` が要求したリポジトリであれば、発行は監査記録に残っています。
ワークフロー実行の ID は、GitHub のワークフロー実行のページの URL（`.../actions/runs/<ワークフロー実行の ID>`）にあります。
監査記録のほかの項目は [Lambda Design](../design/lambda.md) の §7 にあります。

ここまでで、デプロイは完了です。
許可オーナーのほかのリポジトリで GATES を使うときは、§8 の接続情報をそのリポジトリから参照できるようにし、ワークフロー利用者に、接続情報の名前、インストール対象、参照する Action のコミットを伝えます。
ワークフロー利用者がワークフローを書く手順は、[Using GATES in Workflows](workflow-usage.md) にあります。

## 10. References

- [GitHub — Registering a GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app)
- [GitHub — Installing your own GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-your-own-github-app)
- [GitHub — OpenID Connect reference](https://docs.github.com/en/actions/reference/security/oidc)
- [GitHub — REST API endpoints for GitHub Actions OIDC](https://docs.github.com/en/rest/actions/oidc)
- [GitHub CLI — gh secret set](https://cli.github.com/manual/gh_secret_set)
- [GitHub CLI — gh variable set](https://cli.github.com/manual/gh_variable_set)
- [AWS — Create an OpenID Connect (OIDC) identity provider in IAM](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc.html)
- [AWS — Configuring reserved concurrency for a function](https://docs.aws.amazon.com/lambda/latest/dg/configuration-concurrency.html)
