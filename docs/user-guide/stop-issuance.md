# Stopping GATES

運用者に向けて、悪用が疑われるときに、GATES の発行や GitHub App としての署名を止める手順を示します。

## 1. Choosing How to Stop

疑いの種類で、最初に取る手段が決まります。

- **GATES を通した不正な発行を疑う**（例：`pull_request_target` のジョブが Pull Request のコードを実行している）：§2 で Lambda の呼び出しを止める
- **GATES を経ない署名を疑う**（例：AWS で KMS Key や Lambda を書き換えられる主体が、署名を悪用している）：§3 で KMS Key を無効にし、続けて §5 で GitHub から秘密鍵を削除する

| 手段 | 止まるもの | 戻せる主体 |
|---|---|---|
| Reserved Concurrency を 0 にする（§2） | GATES の発行だけ。KMS Key での署名は止まらない | Lambda の設定を変えられる主体 |
| KMS Key を無効にする（§3） | 使用中の鍵での署名と、GATES の発行 | `kms:EnableKey` を持つ主体 |
| Installation を停止する（§4） | GitHub App による許可オーナーのリソースへのアクセスと、GATES の発行 | 停止した側だけ |
| GitHub で秘密鍵を削除する（§5） | 削除した鍵での認証と、GATES の発行 | 戻せない。鍵の更新で再開する |

KMS Key の無効化は、AWS の権限を持つ主体が元に戻せます。
**署名の悪用を AWS の権限に左右されずに止められるのは、GitHub での秘密鍵の削除です。**

Reserved Concurrency を 0 にしても KMS Key を無効にしても、止める前に発行されたトークンは、期限（発行から 1 時間）まで使えます。
**止めても、トークンで行われた変更は取り消されません。**
書き込みの権限を持つトークンが悪用された疑いがあれば、§6 で残った変更を確かめます。

## 2. Setting Reserved Concurrency to 0

Lambda の関数のすべての呼び出しが拒否され、GATES の発行が止まります。

```bash
aws lambda put-function-concurrency --function-name gates --reserved-concurrent-executions 0
```

以後、Action は `the Lambda Invoke failed: AWS error code "TooManyRequestsException"` で失敗します。
この呼び出しは、監査記録に残りません。

この変更は Terraform の外で行うため、止めているあいだに別の理由で適用すると、発行が再開します。
長く止めるときは、`terraform/local/terraform.tfvars` の `lambda_reserved_concurrency` も `0` にして適用します。

再開するときは、`lambda_reserved_concurrency` を元の値にして、`make tf-plan` と `make tf-apply` で適用します。
[Deployment](deployment.md) の §9.1 のワークフローで、発行が成功することを確かめます。

## 3. Disabling the KMS Key

使用中の鍵を無効にすると、GATES も、ほかの主体も、その KMS Key で署名できなくなります。

```bash
aws lambda get-function --function-name gates \
  --query 'Configuration.Environment.Variables.GATES_KMS_KEY_ARN' --output text
aws kms disable-key --key-id <KMS Key の ARN>
aws kms describe-key --key-id <KMS Key の ARN> --query 'KeyMetadata.KeyState' --output text
```

状態が `Disabled` であることを確かめます。
以後、Action は `error code "internal_error"` で失敗します。

[Key Rotation](rotate-key.md) の途中で KMS Key が 2 つあるときは、使用中でない KMS Key にも、GitHub App に登録された秘密鍵がインポートされていることがあります。
その KMS Key でも署名できるため、同じコマンドで無効にします。
KMS Key の ARN は、Output の `kms_key_arns` で確かめます。

```bash
jq -r '.outputs.kms_key_arns.value' terraform/local/terraform.tfstate
```
無効にする前に署名された GitHub App JWT は、その期限（最長 10 分）まで GitHub に受け付けられ得ます。

KMS Key の状態は Terraform の Plan に現れないため、適用しても無効のままです。
再開するときは、`aws kms enable-key --key-id <KMS Key の ARN>` で有効にします。

## 4. Suspending the Installation

Installation を停止すると、GitHub App は許可オーナーのリソースにアクセスできなくなります。
Installation の設定画面（ユーザーアカウントなら `Settings` → `Applications` → `Installed GitHub Apps` の `Configure`）で `Suspend` をクリックします。
以後、Action は `error code "installation_unavailable"` で失敗します。

停止を解けるのは、停止した側だけです。
設定画面で停止した場合は、同じ画面の `Unsuspend` で解きます。
GATES の設定を変えずに、発行が再開します。

## 5. Deleting the Private Key from GitHub

削除した鍵で署名した GitHub App JWT は、GitHub App としての認証に使えなくなります。
**削除は戻せないため、再開には鍵の更新が要ります。**

使用中の鍵に対応する秘密鍵を、公開鍵のフィンガープリントで見分けます。
次のコマンドの出力と、GitHub App の `Credentials` → `Key pairs` に表示される `SHA256:` の後の値を比べます。

```bash
aws kms get-public-key --key-id <KMS Key の ARN> --query PublicKey --output text \
  | openssl base64 -d -A | openssl sha256 -binary | openssl base64
```

GitHub は最後の鍵を削除させないため、`Key pairs` に鍵が一つしかなければ、先に `New key` で新しい鍵を生成します。
すぐに鍵の更新に使わないなら、ダウンロードされたファイルは削除し、後の鍵の更新で改めて鍵を生成します。
そのうえで、対応する鍵を `Delete` します。

## 6. Investigating

KMS の署名の記録を CloudTrail から引き、主体ごとに数えます。
`<開始>` と `<終了>` は `YYYY-MM-DDThh:mm:ssZ` の形の時刻です。

```bash
aws cloudtrail lookup-events --region <KMS Key のリージョン> \
  --lookup-attributes AttributeKey=EventName,AttributeValue=Sign \
  --start-time <開始> --end-time <終了> \
  --query 'Events[].CloudTrailEvent' --output json \
  | jq -r '.[] | fromjson | [.userIdentity.arn, .resources[0].ARN] | @tsv' | sort | uniq -c
```

GATES の Lambda の署名の主体は、`arn:aws:sts::<AWS アカウント ID>:assumed-role/gates-lambda/<セッション名>` です。
これと違う主体の署名があれば、GATES を経ない署名が疑われます。
GATES を通した発行は、監査記録の `caller.event_name` と `caller.run_id` から、起動したワークフローをたどって調べます。

疑わしい発行の監査記録の `request.permissions` に `write` か `admin` があれば、`request.repositories` のリポジトリごとに、トークンで残された変更を確かめます。
GATES を経ない署名を疑うときは、GitHub App に付与した書き込みの権限で、インストール対象のすべてのリポジトリが対象になります。
次のコマンドは、リポジトリの管理者の権限で `gh auth login` した GitHub CLI で実行します。

```bash
gh api repos/<オーナー名>/<リポジトリ名>/keys --jq '.[] | [.id, .title, .created_at, .read_only] | @tsv'
gh api repos/<オーナー名>/<リポジトリ名>/hooks --jq '.[] | [.id, .config.url, .created_at] | @tsv'
gh api "repos/<オーナー名>/<リポジトリ名>/collaborators?affiliation=direct" --jq '.[] | [.login, .role_name] | @tsv'
gh api repos/<オーナー名>/<リポジトリ名>/branches --jq '.[].name'
gh api "repos/<オーナー名>/<リポジトリ名>/commits?path=.github/workflows&since=<開始>" --jq '.[] | [.sha, .commit.committer.date, .commit.author.name] | @tsv'
```

- **デプロイキー、Webhook、コラボレーター**：身に覚えのないものは削除する。トークンが失効しても、これらを通したアクセスは続く
- **ブランチ**：身に覚えのないブランチは、内容を確かめてから削除する
- **ワークフローのファイル**：既定のブランチのワークフローのファイルを変えたコミットを確かめる。書き換えられたワークフローは、そのリポジトリの Secrets と、そのリポジトリを信頼するクラウドの権限を使えるため、その実行の記録も確かめる

## 7. Resuming after Disabling the Key or Deleting It

**再開する前に、悪用した主体の AWS の権限を取り除きます。**
権限が残ったままでは、新しい KMS Key での署名も悪用され得るためです。

§3 か §5 で止めた後は、[Key Rotation](rotate-key.md) の手順で新しい鍵に替えて再開します。
そのとき、次のように読み替えます。

- Key Rotation の §2 のうち、ワークフローが成功することは満たせない。Key Rotation の §5 で新しい鍵に切り替えるまで、発行は失敗したままになる
- 旧鍵への切り戻しは行わない
- Key Rotation の §7 の旧 KMS Key の無効化と、§8 の旧秘密鍵の削除は、済んでいれば省く

## 8. References

- [GitHub — Suspending a GitHub App installation](https://docs.github.com/en/apps/maintaining-github-apps/suspending-a-github-app-installation)
- [GitHub — Managing private keys for GitHub Apps](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps)
- [AWS — Logging AWS KMS API calls with AWS CloudTrail](https://docs.aws.amazon.com/kms/latest/developerguide/logging-using-cloudtrail.html)
