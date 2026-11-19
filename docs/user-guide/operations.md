# Operations

運用者に向けて、デプロイの後に行う作業の手順を示します。

| 節 | 作業 | 行う時期 |
|---|---|---|
| §1 | インストール対象にリポジトリを足す、外す | 読ませたいリポジトリが変わったとき |
| §2 | 監査記録を確かめる | 定期的に。失敗や悪用の疑いを調べるとき |
| §3 | `terraform.tfvars` の値を変える | Audience、同時実行数、保持期間を変えるとき。許可オーナーが改名したとき |
| §4 | GATES を新しいリリースに更新する | 新しいリリースが出たとき |
| §5 | 鍵の更新を行う | 定期の更新の時期。鍵の漏えいが疑われるとき |
| §6 | 発行や署名を止める | 悪用が疑われるとき |

Terraform の操作は、[Deployment](deployment.md) の §2.1 と §2.2 の前提のもとで、`terraform/local/` を保管した GATES のリポジトリのルートから実行します。

## 1. Managing the Installation Scope

### 1.1 Adding and Removing Repositories

インストール対象は、GitHub App とオーナーの管理者が GitHub の画面で変えます。
GATES の設定は変えません。

1. Installation の設定画面を開く。ユーザーアカウントなら `Settings` → `Applications` → `Installed GitHub Apps` で、Organization なら Organization の `Settings` → `GitHub Apps` で、GitHub App の `Configure` をクリックする
2. `Repository access` が `Only select repositories` であることを確かめる
3. `Select repositories` で、リポジトリを加えるか外す
4. `Save` をクリックする

変更は、1 分ほどで発行に反映されます。
加えてよいリポジトリの考え方は、[Security Considerations](security-considerations.md) の §1.1 にあります。

**外したリポジトリを要求するワークフローは、ほかのリポジトリと一緒に要求していても、要求全体が失敗します。**
外す前に、そのリポジトリを要求しているワークフローを §2.2 のクエリで調べ、ワークフロー利用者に直してもらいます。

インストール対象は読まれる側のリポジトリで、GATES を呼び出す側とは別です。
新しいリポジトリのワークフローから GATES を呼び出せるようにするには、そのリポジトリに接続情報を登録します（Deployment の §8）。

### 1.2 Permissions of the GitHub App

GitHub App の権限は、`Contents` の `Read-only` から広げません。
GATES が発行するトークンの権限は広がりませんが、GATES を経ずに署名された場合の上限が広がります。
Installation の設定画面に `Permission updates requested. Review request.` が出ても、身に覚えのない要求は承認しません。

## 2. Reviewing Audit Records

### 2.1 Where the Records Are

監査記録は、CloudWatch Logs のロググループ `/aws/lambda/gates` の、`msg` が `audit record` の行です。
保持期間は `log_retention_in_days` で決まり、過ぎた記録は探せません。
主なキーは次のとおりです。

| キー | 意味 |
|---|---|
| `time` | 結果が確定した時刻（UTC） |
| `request_id` | 要求ごとの ID。Action の注釈とログに出る値と同じ |
| `result` | 成功なら `success`、失敗ならエラーコード |
| `reason` | 失敗した理由。応答と注釈には出ない |
| `caller.run_id` | 呼び出し元のワークフロー実行の ID。`.../actions/runs/<ID>` の URL でたどれる |
| `caller.event_name` | ワークフローを起動したイベント |
| `caller.workflow_ref` | 呼び出し元のワークフローのファイル（`<オーナー名>/<リポジトリ名>/.github/workflows/<ファイル>@<ref>`） |
| `request.repositories` | 要求したリポジトリ。要求の表記のまま記録される |
| `claims_set.head_ref` | Pull Request の元のブランチ名 |

`caller` は、GATES が呼び出し元を確かめた後にだけ記録されます。
`invalid_request` や `invalid_id_token` のような早い段階の失敗は、`caller` で絞るクエリには現れません。

### 2.2 Common Queries

`<開始>` と `<終了>` は、ミリ秒の UNIX 時刻です。

期間の中で失敗した要求を出します。
同じ `result` が続いていれば、[Troubleshooting](troubleshooting.md) の §4 で原因を引きます。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --filter-pattern '{ $.msg = "audit record" && $.result != "success" }' \
  --query 'events[].message' --output json \
  | jq -r '.[] | fromjson | [.time, .request_id, .result, .reason] | @tsv'
```

特定のリポジトリを要求したワークフローを出します。
名前の大文字と小文字は区別しません。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --filter-pattern '{ $.msg = "audit record" }' \
  --query 'events[].message' --output json \
  | jq -r --arg repo '<オーナー名>/<リポジトリ名>' '.[] | fromjson
      | select((.request.repositories // []) | map(ascii_downcase) | index($repo | ascii_downcase))
      | [.time, .result, .caller.workflow_ref, .caller.run_id] | @tsv'
```

`pull_request_target` と `workflow_run` で起動したジョブへの発行を出します。
見つかったら、そのジョブが信頼できないコードを実行していないかを、ワークフロー実行をたどって確かめます。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --filter-pattern '{ $.msg = "audit record" && $.result = "success" && ($.caller.event_name = "pull_request_target" || $.caller.event_name = "workflow_run") }' \
  --query 'events[].message' --output json \
  | jq -r '.[] | fromjson
      | [.time, .caller.event_name, .caller.workflow_ref, .caller.run_id, .claims_set.head_ref // ""] | @tsv'
```

### 2.3 Records Outside GATES

IAM が拒否した呼び出しは GATES に届かないため、AWS CloudTrail で調べます。
IAM Role の引き受けは既定で記録されます。
Lambda の呼び出しは、CloudTrail のデータイベントの記録を有効にしたときだけ記録されます。

## 3. Changing Settings

### 3.1 Applying a Change

`terraform/local/terraform.tfvars` の値を変えたら、次の順に適用します。

1. いまデプロイしているリリースのタグをチェックアウトしてあることを確かめる。別のリリースのソースでは、関数のコードの差分も Plan に出る
2. `make tf-plan` が、§3.2 から §3.4 に書いた差分だけを示すことを確かめる
3. `make tf-apply` で適用し、もう一度 `make tf-plan` を実行して `No changes` を確かめる
4. Deployment の §9.1 のワークフローを実行し、発行が成功することを確かめる

```bash
make tf-plan
make tf-apply
make tf-plan
```

`kms_keys` と `active_kms_key` は、§5 の鍵の更新の手順で変えます。

### 3.2 Audience

`gates_audience` を変えたら、ワークフローが参照する Variable の `GATES_AUDIENCE` も同じ値に変えます。
GATES は Audience を一つしか受け付けないため、適用と Variable の変更のあいだに動いたワークフローは `error code "invalid_id_token"` で失敗します。
ワークフローが動かない時間帯に、続けて行います。

Plan は、`aws_lambda_function.gates` の環境変数 `GATES_AUDIENCE` の変更だけを示します。

### 3.3 Reserved Concurrency and Log Retention

`lambda_reserved_concurrency` を変えると、Plan は `aws_lambda_function.gates` の `reserved_concurrent_executions` の変更だけを示します。

`log_retention_in_days` を変えると、Plan は `aws_cloudwatch_log_group.lambda` の `retention_in_days` の変更だけを示します。
**保持期間を短くすると、新しい期間を過ぎた記録は削除され、後から延ばしても戻りません。**

### 3.4 Owner Name after a Rename

許可オーナーの名前が変わったら、`allowed_owner_name` を新しい名前にして適用し直します。
適用し直すまで、Action は `AssumeRoleWithWebIdentity failed` で失敗し、発行は止まります。
新しい名前は Deployment の §5.1 のコマンドで調べます。

Plan は、`aws_iam_role.github_actions` の Trust Policy と、`aws_lambda_function.gates` の環境変数 `GATES_ALLOWED_OWNER_NAME` の変更だけを示します。
ワークフローの `repositories` に古い名前が残っていれば、ワークフロー利用者に直してもらいます。

### 3.5 Values Fixed at Deployment

`allowed_owner_id`、`allowed_owner_type`、`github_app_client_id` は、デプロイのときに決めて変えない値です。
許可オーナーや GitHub App を別のものに変える手順は用意していません。

## 4. Updating GATES

### 4.1 Order of the Update

Lambda の関数と、ワークフローが参照する Action は、同じリリースにそろえます。
Lambda を先に更新し、ワークフロー利用者が Action の参照先を後から更新します。
そろうまでのあいだは、ワークフローが失敗することがあるため、更新の時期を事前にワークフロー利用者に伝えておきます。

### 4.2 Before the Update

リリースのページ（`https://github.com/tmknom/gates/releases`）で、更新先の変更点を読みます。
Terraform の変数の追加は、次の差分でも確かめられます。
加わった変数があれば、`terraform/local/terraform.tfvars` に値を足します。

```bash
git fetch --tags
git diff <更新前のバージョン> <更新後のバージョン> -- terraform/terraform.tfvars.example
```

更新先の配布物をダウンロードし、[Release Verification](verify-release.md) の §2 と §3 で検証します。
検証に失敗したリリースには更新しません。

### 4.3 Applying the New Release

更新先のタグをチェックアウトし、Plan を確かめます。

```bash
git switch --detach <更新後のバージョン>
make tf-plan
```

Terraform の定義が変わっていないリリースなら、Plan は `aws_lambda_function.gates` の `source_code_hash` の変更だけを示します。
リリースの変更点にない差分が現れたら、適用しません。

`make tf-plan` が `Instance cannot be destroyed` で止まったら、Plan が KMS Key の削除か置換を含んでいます。
出力で `aws_kms_external_key.signing` の `# forces replacement` の行を探し、原因の値を元に戻します。
**`prevent_destroy` を外して適用してはいけません。** KMS Key を削除すると、インポートした鍵素材も失われます。

Plan に問題がなければ、適用します。

```bash
make tf-apply
make tf-plan
```

### 4.4 Checking the Updated Deployment

Deployment の §6.3 のコマンドで、関数のコードが更新先の配布物と同じであることを確かめます。
続けて、Deployment の §9.1 のワークフローの Action の参照先を、更新先のタグのコミットの SHA（`git rev-parse HEAD`）に変えて実行し、成功を確かめます。
成功したら、ワークフロー利用者に、Action の参照先を同じコミットに更新するよう伝えます。

### 4.5 Returning to the Previous Release

更新の後に問題が見つかったら、更新前のタグをチェックアウトし、§4.3 と §4.4 と同じ手順で適用し直します。
Plan に KMS Key の削除や置換が含まれていれば、適用しません。
ワークフロー利用者が Action の参照先を更新済みなら、更新前のコミットに戻してもらいます。

## 5. Rotating the Key

手順は [Key Rotation](rotate-key.md) にあります。
次のときに、鍵の更新を行います。

- **定期の更新**：GitHub App の秘密鍵には有効期限がない。間隔は、自組織の鍵の管理の方針に合わせる
- **秘密鍵のコピーが残った疑い**：インポートのときの秘密鍵のファイルが、削除されずに残ったか、バックアップなどに写った疑いがあるとき
- **鍵を扱う権限を持つ人の交代**：Terraform を適用する権限、Key Policy を書き換える権限、鍵素材をインポートする権限を持っていた人が外れたとき。その人は秘密鍵を手元に持っていたか、KMS Key に自分の署名を許す Grant を作れた。新しい KMS Key には、それまでの Grant はない

外れた人の AWS の権限は、鍵の更新とは別に取り除きます。

## 6. When Abuse Is Suspected

発行や署名を止める手順は、[Stopping GATES](stop-issuance.md) にあります。

## 7. References

- [GitHub — Reviewing and modifying installed GitHub Apps](https://docs.github.com/en/apps/using-github-apps/reviewing-and-modifying-installed-github-apps)
- [AWS — Change log data retention in CloudWatch Logs](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/Working-with-log-groups-and-streams.html)
- [AWS — Filter pattern syntax for metric filters, subscription filters, filter log events, and Live Tail](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/FilterAndPatternSyntax.html)
