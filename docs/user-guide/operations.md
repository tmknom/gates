# Operations

運用者に向けて、デプロイの後に行う作業の手順を示します。

| 作業 | 行う時期 |
|---|---|
| インストール対象と、GitHub App の権限と発行上限を変える | 操作させたいリポジトリか、ワークフローに要る権限が変わったとき |
| 保護対象の一覧を変える | 守りたいリポジトリか、それを要求するワークフローが変わったとき。保護対象か許可 Caller のリポジトリを改名したとき |
| リポジトリポリシーの適用を切り替える | トークンで操作されるリポジトリの管理者に、自分のリポジトリの許可 Caller と権限の上限を決めさせたいとき |
| 監査記録を確かめる | 定期的に。失敗や悪用の疑いを調べるとき |
| `terraform.tfvars` の値を変える | Audience、同時実行数、保持期間を変えるとき。許可オーナーが改名したとき |
| GATES を新しいリリースに更新する | 新しいリリースが出たとき |
| 鍵の更新を行う | 定期の更新の時期。鍵の漏えいが疑われるとき |
| 発行や署名を止める | 悪用が疑われるとき |

Terraform の操作は、デプロイしたときと同じツールと AWS の一時認証情報で、`terraform/local/` を保管した GATES のリポジトリのルートから実行します。

## 1. Managing the Installation Scope and Permissions

### 1.1 Adding and Removing Repositories

インストール対象は、GitHub App とオーナーの管理者が GitHub の画面で変えます。
GATES の設定は変えません。

1. Installation の設定画面を開く。ユーザーアカウントなら `Settings` → `Applications` → `Installed GitHub Apps` で、Organization なら Organization の `Settings` → `GitHub Apps` で、GitHub App の `Configure` をクリックする
2. `Repository access` が `Only select repositories` であることを確かめる
3. `Select repositories` で、リポジトリを加えるか外す
4. `Save` をクリックする

リポジトリポリシーを適用しないデプロイでは、許可オーナーのどのワークフローから操作されてもよいリポジトリだけを加えます。
一部のワークフローからだけ操作させたいリポジトリは、先に §1.3 で保護対象にして適用してから加えます。
保護対象の一覧にないリポジトリは、加えた時点から、許可オーナーのどのワークフローも要求できるためです。

リポジトリポリシーを適用するデプロイでは、加えたリポジトリは、その管理者がリポジトリポリシーを置くまで、どのワークフローからも要求できません。
加えたことを、そのリポジトリの管理者に伝えます。

変更は、1 分ほどで発行に反映されます。

**外したリポジトリを要求するワークフローは、ほかのリポジトリと一緒に要求していても、要求全体が失敗します。**
外す前に、そのリポジトリを要求しているワークフローを監査記録（§2.2）で調べ、ワークフロー利用者に直してもらいます。

インストール対象はトークンで操作される側のリポジトリで、GATES を呼び出す側とは別です。
新しいリポジトリのワークフローから GATES を呼び出せるようにするには、そのリポジトリに接続情報を登録します。

### 1.2 Changing the Permissions

**GitHub App の権限と発行上限の変更は、許可オーナーのすべてのワークフローにできることを変える、認可境界の変更です。**
保護対象でも、許可 Caller が権限の上限の範囲で要求できる権限が変わります。
変更の前に、許可オーナーのどのワークフローにその権限を渡してよいかを判断します。

権限を広げるときは、GitHub App の権限と発行上限の両方を変えます。
どちらか一方だけでは、加えた権限は発行されません。

1. GitHub App の設定ページの `Permissions & events` で、Repository permissions に権限を加え、`Save changes` をクリックする
2. Installation の設定画面に出る `Permission updates requested. Review request.` の `Review request` から、加えた権限を承認する。承認するまで、加えた権限は Installation に付与されない
3. `terraform.tfvars` の `issuable_permissions` に同じ権限を加えて適用する。Plan は、`aws_lambda_function.gates` の環境変数 `GATES_ISSUABLE_PERMISSIONS` の変更だけを示す
4. ワークフロー利用者に、使えるようになった権限を伝える

承認は、GitHub App とオーナーの管理者が行います。
身に覚えのない `Permission updates requested.` は承認しません。

権限を狭めるときは、先に監査記録で、その権限を要求しているワークフローを調べ、ワークフロー利用者に要求から外してもらいます。
`contents:write` を `contents:read` にするように水準を下げるときは、`write` の要求だけを調べます。
外さないまま狭めると、そのワークフローの要求は全体が失敗します。
そのうえで、発行上限から外して適用し、GitHub App の権限からも外します。
発行上限から外すだけでは、GATES を経ない署名で得られる権限は狭まりません。

**発行上限を狭めても、発行済みのトークンは失効しません。**
発行済みのトークンは、期限（発行から 1 時間）まで、発行したときの権限で使えます。
悪用を止めるために狭めるときは、[Stopping GATES](stop-issuance.md) の手段もあわせて検討します。

### 1.3 Changing the Protected Repositories

**保護対象の一覧の変更も、保護対象を要求できるワークフローと権限を変える、認可境界の変更です。**
保護対象を加える、外す、許可 Caller を加える、外す、権限の上限を変える、のどれも同じ手順で行います。
リポジトリポリシーを適用するデプロイでは、保護対象への要求には、そのリポジトリのリポジトリポリシーの許可も要ります。
許可 Caller を加えたら、保護対象のリポジトリの管理者にも、リポジトリポリシーで同じ Caller を許してもらいます。

1. 保護対象を加えるか、許可 Caller を外すか、権限の上限を狭めるときは、先に監査記録（§2.2）で、その保護対象を要求しているワークフローを調べる。許されなくなる要求は、ほかのリポジトリと一緒に要求していても全体が失敗するため、ワークフロー利用者に直してもらうか、そのワークフローのリポジトリを許可 Caller に加える
2. `terraform.tfvars` の `protected_repositories` を変える。書き方と、`make tf-plan` を通っても Lambda の関数が初期化に失敗する誤りは、[Deployment](deployment.md) の「Protected Repositories」にある
3. `make tf-plan` が、`aws_lambda_function.gates` の環境変数 `GATES_PROTECTED_REPOSITORIES` の変更だけを示すことを確かめ、適用する。Plan の差分には、コメントを含めた一覧の全体が出る
4. GATES からトークンを受け取るワークフローを実行し、発行が成功することを確かめる。一覧に Lambda の関数が初期化に失敗する誤りがあれば、どの要求も `the Lambda function returned FunctionError` で失敗するため、[Troubleshooting](troubleshooting.md) で初期化の失敗のログを読んで直す
5. 許可 Caller のワークフロー利用者に、要求できるようになった保護対象と権限の上限を伝える

適用が完了した後の要求から、新しい一覧で判定されます。

**一覧を変えても、発行済みのトークンは失効しません。**
保護対象を加えたときも、許可 Caller を外したときも、それまでに発行したトークンは、期限（発行から 1 時間）まで発行したときの対象と権限で使えます。
悪用を止めるために変えるときは、[Stopping GATES](stop-issuance.md) の手段もあわせて検討します。

### 1.4 Renaming a Protected Repository

保護対象を改名すると、一覧の `name` を新しい名前に直すまで、その保護対象への発行が止まります。
新しい名前の要求は `issuance_verification_failed` に、旧名の要求は `authorization_denied` か `github_issuance_rejected` になります。
新しい名前の要求のたびに、GitHub 側ではトークンが発行され、GATES が失効を試みるため、改名から一覧の更新までの時間を短くします。

1. 改名の時期を、その保護対象を要求するワークフロー利用者と合わせる
2. リポジトリを改名する
3. 一覧の保護対象の `name` を新しい名前にして、§1.3 の手順で適用する。`id` は変えない
4. ワークフロー利用者に、`repositories` を新しい名前に直してもらう

保護対象を削除して同じ名前で作り直したリポジトリは、別の Repository ID を持ちます。
一覧を直すまで、作り直したリポジトリへの要求は、どの Caller からでも `authorization_denied` か `issuance_verification_failed` になります。
保護するなら新しい Repository ID を調べて `id` を直し、保護しないなら一覧から外します。

### 1.5 Renaming an Allowed Caller Repository

許可 Caller のリポジトリを改名すると、一覧の `repository_name` を新しい名前に直すまで、そのリポジトリのワークフローからの保護対象への要求は `authorization_denied` になります。
保護対象でないリポジトリだけの要求は、これまでどおり発行されます。

1. 改名の時期を、そのリポジトリのワークフロー利用者と合わせる
2. リポジトリを改名する
3. 一覧の、そのリポジトリの許可 Caller の `repository_name` を新しい名前にして、§1.3 の手順で適用する。`repository_id` は変えない。同じリポジトリを複数の保護対象の許可 Caller にしていれば、すべて直す

許可 Caller のリポジトリを削除して同じ名前で作り直したときも、新しいリポジトリは許可を引き継がず、要求は `authorization_denied` になります。
許可するなら、新しい Repository ID を調べて `repository_id` を直します。

### 1.6 Switching to Repository Policy Enforcement

**切り替えた時点で、リポジトリポリシーのないリポジトリへの発行がすべて止まります。**
拒否せずに記録だけする試行の段階はないため、先にリポジトリポリシーを置いてから切り替えます。

1. インストール対象のリポジトリの管理者に、[Repository Policy](repository-policy.md) を示して、切り替えの時期を伝える。保護対象にも、リポジトリポリシーが要る
2. GitHub App の設定ページの `Permissions & events` で、Repository permissions の `Single file` を `Read-only` にし、パスに `.github/gates-policy.yml` だけを書いて、`Save changes` をクリックする
3. Installation の設定画面の `Review request` から、`Single file` を承認する。承認しないまま切り替えると、認可を通った要求はどれも `github_issuance_rejected` になる
4. 各リポジトリの管理者に、既定ブランチにリポジトリポリシーを置いてもらう。置いたかは、リポジトリごとに次のコマンドが `file` を出力することで確かめる

    ```bash
    gh api repos/<オーナー名>/<リポジトリ名>/contents/.github/gates-policy.yml --jq .type
    ```

5. 監査記録（§2.2）で、11 件以上のリポジトリを一度に要求しているワークフローがないかを確かめる。あれば、ワークフロー利用者に要求を 10 件以下に分けてもらう
6. `terraform.tfvars` の `repository_policy_enforcement` を `true` にする
7. `make tf-plan` が、`aws_lambda_function.gates` の環境変数 `GATES_REPOSITORY_POLICY_ENFORCEMENT` の `"false"` から `"true"` への変更だけを示すことを確かめ、適用する
8. リポジトリポリシーを置いたリポジトリを要求するワークフローを実行し、発行が成功することを確かめる

**適用しないデプロイに戻ると、保護対象でないリポジトリは、再び許可オーナーのどのワークフローからも要求できるようになります。**
切り戻す前に、インストール対象を、許可オーナーのどのワークフローから操作されてもよいリポジトリと保護対象だけにします。
そのうえで、`repository_policy_enforcement` を `false` にし、`make tf-plan` が `aws_lambda_function.gates` の環境変数 `GATES_REPOSITORY_POLICY_ENFORCEMENT` の `"true"` から `"false"` への変更だけを示すことを確かめて適用します。
GitHub App の `Single file` は、適用しないデプロイでは使わないため、戻したまま使い続けるなら外します。

切り替えた後は、リポジトリポリシーの変更も認可境界の変更です。
ただし、変更するのは既定ブランチに書き込める人と、`contents:write` か `administration:write` を許された Caller で、運用者のレビューと Plan を経ません。
リポジトリポリシーで越えさせたくない上限があるリポジトリは、保護対象にして、保護対象の一覧で上限を決めます。

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
| `request.permissions` | 要求した権限。権限名をキー、水準を値とするオブジェクト（`{"contents":"write","pull_requests":"read"}` など） |
| `revocation` | GitHub が発行したトークンを照合で返さなかったときの、失効の結果（`succeeded`、`failed`、`not_attempted`） |
| `policy_read_revocation` | リポジトリポリシーを適用するデプロイで、リポジトリポリシーを読むために GATES が取得したトークンの失効の結果（`succeeded`、`failed`、`not_attempted`）。GATES がそのトークンの発行を GitHub に求めなかった要求にはない |
| `claims_set.head_ref` | Pull Request の元のブランチ名 |
| `claims_set.repository` | 呼び出し元のリポジトリの完全名。一覧の許可 Caller の `repository_name` と見比べるのに使う |

`caller` は、GATES が呼び出し元を確かめた後にだけ記録されます。
`invalid_request` や `invalid_id_token` のような早い段階の失敗は、`caller` で絞るクエリには現れません。

### 2.2 Common Queries

`<開始>` と `<終了>` は、ミリ秒の UNIX 時刻です。

期間の中で失敗した要求を出します。
同じ `result` が続いていれば、[Troubleshooting](troubleshooting.md) でエラーコードから原因を引きます。

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

特定の権限を要求したワークフローを、要求した水準とともに出します。
発行上限や GitHub App の権限を狭める前に、その権限を使っているワークフローを調べるのに使います。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --filter-pattern '{ $.msg = "audit record" }' \
  --query 'events[].message' --output json \
  | jq -r --arg name '<権限名>' '.[] | fromjson
      | select((.request.permissions // {}) | has($name))
      | [.time, .result, .request.permissions[$name], .caller.workflow_ref, .caller.run_id] | @tsv'
```

`read` でない水準（`write` と `admin`）を一つでも含む要求を出します。
書き込みの権限を得たワークフローと、その実行を確かめるのに使います。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --filter-pattern '{ $.msg = "audit record" }' \
  --query 'events[].message' --output json \
  | jq -r '.[] | fromjson
      | select((.request.permissions // {}) | any(.[]; . != "read"))
      | [.time, .result, (.request.permissions | to_entries | map("\(.key):\(.value)") | join(",")), .caller.workflow_ref, .caller.run_id] | @tsv'
```

保護対象の条件で拒否した要求を出します。
`reason` の `protected repositories[<J>]` は、一覧の `repositories` の 0 から数えた位置で、続く文言が拒否の理由を示します。
理由の読み方は、[Troubleshooting](troubleshooting.md) の `authorization_denied` にあります。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --filter-pattern '{ $.msg = "audit record" && $.result = "authorization_denied" }' \
  --query 'events[].message' --output json \
  | jq -r '.[] | fromjson
      | select(.reason | contains("matches protected repositories"))
      | [.time, .reason, .claims_set.repository, .caller.workflow_ref, .caller.run_id] | @tsv'
```

リポジトリポリシーで拒否した要求を出します。
`reason` の `repositories[<I>]` は `request.repositories` の 0 から数えた位置で、続く文言が拒否の理由（ファイルがない、不正、Caller が許されていない、権限の上限を超える）を示します。
リポジトリポリシーの内容は記録されないため、理由が示す誤りがないか、そのリポジトリの管理者にリポジトリポリシーを確かめてもらいます。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --filter-pattern '{ $.msg = "audit record" && $.result = "authorization_denied" }' \
  --query 'events[].message' --output json \
  | jq -r '.[] | fromjson
      | select(.reason | test("repositories\\[[0-9]+\\]: (repository policy|permissions are not within the permission limit of the repository policy)"))
      | [.time, .reason, (.request.repositories | join(",")), .claims_set.repository, .caller.run_id] | @tsv'
```

11 件以上のリポジトリを一度に要求したワークフローを出します。
リポジトリポリシーを適用するデプロイでは、これらの要求は `authorization_denied` になります。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --filter-pattern '{ $.msg = "audit record" }' \
  --query 'events[].message' --output json \
  | jq -r '.[] | fromjson
      | select((.request.repositories // []) | length > 10)
      | [.time, .result, (.request.repositories | length), .caller.workflow_ref, .caller.run_id] | @tsv'
```

保護対象の名前と Repository ID が一覧と食い違って、トークンを返さなかった要求を出します。
保護対象の改名か作り直しの後に一覧を直していないか、一覧の `id` か `name` を書き誤ったことを示します。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --filter-pattern '{ $.msg = "audit record" && $.result = "issuance_verification_failed" }' \
  --query 'events[].message' --output json \
  | jq -r '.[] | fromjson
      | select(.reason | contains("protected repositories["))
      | [.time, .reason, .revocation, (.request.repositories | join(",")), .caller.run_id] | @tsv'
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
2. `make tf-plan` が、変えた値について以下の節に書いた差分だけを示すことを確かめる
3. `make tf-apply` で適用し、もう一度 `make tf-plan` を実行して `No changes` を確かめる
4. GATES からトークンを受け取るワークフローを実行し、発行が成功することを確かめる

```bash
make tf-plan
make tf-apply
make tf-plan
```

`kms_keys` と `active_kms_key` は、[Key Rotation](rotate-key.md) の手順で変えます。
`issuable_permissions` は、§1.2 の手順で GitHub App の権限とあわせて変えます。
`repository_policy_enforcement` は、§1.6 の手順で変えます。

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

更新先の配布物をダウンロードし、[Release Verification](verify-release.md) の手順でチェックサムと Attestation を検証します。
検証に失敗したリリースには更新しません。

保護対象の一覧を持たないリリースから更新するときは、次を確かめます。

- **`protected_repositories` が必須になる**：既定値がないため、`terraform.tfvars` に書くまで `make tf-plan` が止まる。保護対象を置かないなら `repositories: []` を書く。書き方は [Deployment](deployment.md) の「Protected Repositories」にある。Plan は、`aws_lambda_function.gates` の `source_code_hash` に加えて、環境変数 `GATES_PROTECTED_REPOSITORIES` の追加を示す
- **発行上限の高い水準が、低い水準の要求も許すようになる**：`contents:write` だけを含めた発行上限では、これまで拒否していた `contents:read` の要求も発行されるようになる。高い水準だけを含めた権限名ごとに、低い水準の要求が発行されてよいかを確かめる。`contents:read` と `contents:write` の両方を書いた発行上限は、`contents:write` だけを書いたものと同じ意味になる

リポジトリポリシーを持たないリリースから更新するときは、次を確かめます。

- **`repository_policy_enforcement` が必須になる**：既定値がないため、`terraform.tfvars` に書くまで `make tf-plan` が止まる。これまでと同じ動作にするなら `false` を書く。`true` にするなら、更新を終えてから §1.6 の手順で切り替える。Plan は、`aws_lambda_function.gates` の `source_code_hash` に加えて、環境変数 `GATES_REPOSITORY_POLICY_ENFORCEMENT` の追加と、`timeout` の 45 から 75 への変更を示す

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

[Release Verification](verify-release.md) の手順で、関数のコードが更新先の配布物と同じであることを確かめます。
続けて、GATES からトークンを受け取るワークフローの Action の参照先を、更新先のタグのコミットの SHA（`git rev-parse HEAD`）に変えて実行し、成功を確かめます。
成功したら、ワークフロー利用者に、Action の参照先を同じコミットに更新するよう伝えます。

### 4.5 Returning to the Previous Release

更新の後に問題が見つかったら、更新前のタグをチェックアウトし、更新と同じ手順で適用し直します。
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

- [Security Considerations](security-considerations.md)：インストール対象に加えてよいリポジトリと、付与を避ける権限
- [GitHub — Reviewing and modifying installed GitHub Apps](https://docs.github.com/en/apps/using-github-apps/reviewing-and-modifying-installed-github-apps)
- [GitHub — Modifying a GitHub App registration](https://docs.github.com/en/apps/maintaining-github-apps/modifying-a-github-app-registration)
- [GitHub — Approving updated permissions for a GitHub App](https://docs.github.com/en/apps/using-github-apps/approving-updated-permissions-for-a-github-app)
- [AWS — Change log data retention in CloudWatch Logs](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/Working-with-log-groups-and-streams.html)
- [AWS — Filter pattern syntax for metric filters, subscription filters, filter log events, and Live Tail](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/FilterAndPatternSyntax.html)
