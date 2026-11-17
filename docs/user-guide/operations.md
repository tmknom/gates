# Operations

GATES（GitHub App Token Exchange System）は、GitHub Actions のワークフローに、指定したリポジトリだけを読める短命なトークンを発行する仕組みです。
ワークフローは GATES の Docker Action（以下、Action）のステップを置き、Action が、運用者の AWS アカウントで動く GATES の AWS Lambda の関数（以下、Lambda）を呼び出してトークンを受け取ります。
Lambda は、GitHub App Private Key（以下、秘密鍵）をインポートした AWS KMS の鍵（KMS Key）で署名して、GitHub にトークンを発行させます。
本文書は、**GATES 運用者**（GATES を AWS にデプロイして運用する人。以下、運用者）が、デプロイの後に繰り返し行う作業と、必要になったときに行う作業の手順を示します。

| 節 | 作業 | 行う時期 |
|---|---|---|
| §1 | インストール対象にリポジトリを足す、外す | 読ませたいリポジトリが変わったとき |
| §2 | 監査記録を確かめる | 定期的に。失敗や悪用の疑いを調べるとき |
| §3 | `terraform.tfvars` の値を変える | Audience、同時実行数、保持期間を変えるとき。許可オーナーが改名したとき |
| §4 | GATES を新しいリリースに更新する | 新しいリリースが出たとき |
| §5 | 鍵の更新を始めるかを決める | 定期の更新の時期、秘密鍵のコピーが残った疑い、鍵を扱う権限を持つ人の交代 |
| §6 | 悪用が疑われるときに発行や署名を止める | 悪用が疑われるとき |

秘密鍵を新しい鍵に替える作業（**鍵の更新**）の手順は、[Key Rotation](rotate-key.md) にあります。
Action の失敗を、注釈やエラーコードから調べる手順は [Troubleshooting](troubleshooting.md) にあります。

## Prerequisites

§3 と §4 の Terraform の操作は、[Deployment](deployment.md) の §2.1 から §2.3 の前提（手元のツール、AWS の一時認証情報とその権限、IAM OIDC Provider）のもとで、GATES のリポジトリのルートから実行します。
デプロイの後も、Deployment の §6.5 のとおり `terraform/local/` を保管してあることが前提です。
`terraform/local/` には、`terraform.tfvars` と、Terraform が作ったリソースを記録した State があります。

AWS を読むだけのコマンド（§2 と §6 の確認）は、GATES を適用した AWS アカウントの、ログと設定と CloudTrail の記録を読める一時認証情報で実行します。
§6.2 と §6.3 での停止と、§6.3 での再開には、Lambda の関数の同時実行数と KMS Key の状態を変えられる一時認証情報が要ります。
§6.2 での再開は Terraform の適用で行うため、§3 と同じ前提で行います。
AWS CLI のリージョン（`AWS_REGION`）は、GATES を適用したリージョンにしておきます。

## 1. Managing the Installation Scope

### 1.1 What the Installation Scope Decides

GATES は、運用者が作った GitHub App としてトークンを発行します。
GATES がトークンを発行するのは、**許可オーナー**（GATES がトークンを発行してよいと運用者が設定した GitHub のユーザーアカウントか Organization）のワークフローだけです。
GitHub App の **インストール対象**（GitHub App をオーナーにインストールするときに選ぶ、GitHub App がアクセスできるリポジトリ）は、許可オーナーへのインストールで選びます。
許可オーナーのワークフローは、どのリポジトリで動いていても、インストール対象のどのリポジトリのトークンも受け取れます。
GATES は呼び出し元を許可オーナー単位でしか区別しないためです。
このため、インストール対象が、許可オーナーのワークフローから読まれてよい範囲になります。
どのリポジトリを加えてよいかの考え方は、[Security Considerations](security-considerations.md) の §2 にあります。

インストール対象は、読まれる側のリポジトリを決めるもので、GATES を呼び出す側のリポジトリとは別です。
許可オーナーの新しいリポジトリのワークフローから GATES を呼び出せるようにするには、インストール対象に加えるのではなく、そのリポジトリから **接続情報**（Action が GATES を呼び出すのに使う、AWS アカウント ID、IAM Role、関数名、リージョン、Audience の値）を参照できるようにします。
登録の手順は、Deployment の §8 にあります。

### 1.2 Adding and Removing Repositories

インストール対象の変更は、**GitHub App とオーナーの管理者**（GitHub App のインストール先と、オーナーの GitHub の設定を管理する人）が GitHub の画面で行います。
ユーザーアカウントを許可オーナーにする構成では、運用者が兼ねることが多い役割です。
GATES の設定は変えません。
GATES はインストール対象を保持せず、発行のたびに GitHub が判定するためです。

1. Installation（GitHub App をオーナーにインストールした単位）の設定画面を開く。許可オーナーがユーザーアカウントなら、`Settings` → `Applications` → `Installed GitHub Apps` で GitHub App の `Configure` をクリックする。Organization なら、Organization の `Settings` → `GitHub Apps`（`Third-party Access` の下）で `Configure` をクリックする
2. `Repository access` が `Only select repositories` であることを確かめる
3. `Select repositories` で、リポジトリを加えるか外す
4. `Save` をクリックする

GATES の検証環境では、リポジトリを加えた変更も外した変更も、保存から約 1 分後の発行の要求に反映されていました（[GitHub App Installation Changes](../research/github/installation-changes.md)）。
これより短い間隔は確かめていません。

インストール対象から外したリポジトリを要求すると、GitHub がトークンの発行を拒否し、Action は `error code "github_issuance_rejected"` を含む注釈で失敗します。
外したリポジトリをほかのリポジトリと一緒に要求した場合も、残りのリポジトリだけのトークンにはならず、要求全体が失敗します。
リポジトリを外すときは、そのリポジトリを要求しているワークフローを先に直すよう、**ワークフロー利用者**（GATES からトークンを受け取るワークフローを書く人）に伝えます。
どのワークフローがそのリポジトリを要求しているかは、§2.3 の要求したリポジトリで引くクエリで調べられます。

許可オーナーが Organization の場合は、Organization のオーナーのほかに、リポジトリ管理者も自分のリポジトリをインストール対象に加えられます。
誰がインストール対象を変えられるかを管理することは、GATES の前提です（Security Considerations の §2）。
Organization を許可オーナーにした構成は、GATES の検証環境では確かめていません。

### 1.3 Permissions of the GitHub App

GitHub App に権限を足しても、GATES が発行するトークンの権限は `contents:read`（と、GitHub が加えることがある `metadata:read`）のままです。
その理由と検証環境での観測は、Security Considerations の §2 にあります。

GitHub App の権限は、`Contents` の `Read-only` から広げません。
GATES を経ずに GitHub App として署名された場合、得られるトークンの権限の上限は GitHub App の権限になるためです（Security Considerations の §2 と §4）。
GitHub App の設定で権限が足されると、Installation の設定画面に `Permission updates requested. Review request.` が表示されます。
身に覚えのない要求は承認しません。

## 2. Reviewing Audit Records

### 2.1 Where the Records Are

Lambda は、要求ごとに **監査記録**（呼び出し元、要求、結果を、トークンを含めずに書いた 1 行の JSON）を、CloudWatch Logs のロググループ `/aws/lambda/gates` に残します。
監査記録は、同じロググループのほかの行と、`msg` が `audit record` であることで見分けます。
Lambda が応答を返した要求は、成功でも失敗でも監査記録を 1 件残します。

監査記録の保持期間は、`terraform.tfvars` の `log_retention_in_days` で決まり、既定は 7 日です。
保持期間を過ぎた記録は探せないため、自組織の監査の方針が求める期間より短ければ、§3.3 の手順で延ばします。

### 2.2 Keys of the Audit Record

監査記録の主なキーは次のとおりです。
すべてのキーと記録する条件は [Lambda Design](../design/lambda.md) の §7 にあります。
記録する条件を満たさないキーは、空の値ではなく、キーごと省かれます。

| キー | 意味 |
|---|---|
| `time` | 結果が確定した時刻（UTC） |
| `request_id` | 要求ごとの ID。Action の注釈とログに出る値と同じで、監査記録とワークフロー実行を結び付ける |
| `result` | 成功なら `success`、失敗ならエラーコード |
| `reason` | 失敗した理由。満たさなかった条件や、失敗した外部の呼び出しの段階とステータスが入る。応答と注釈には出ない |
| `caller.owner_id`、`caller.repository_id` | 呼び出し元のオーナーとリポジトリの、名前を変えても変わらない数値の ID |
| `caller.run_id`、`caller.run_attempt` | 呼び出し元のワークフロー実行の ID と、その試行の番号。GitHub のワークフロー実行のページの URL（`.../actions/runs/<ワークフロー実行の ID>`）にたどれる |
| `caller.event_name`、`caller.ref`、`caller.sha` | ワークフローを起動したイベントの種別、ref、コミット |
| `caller.workflow_ref`、`caller.job_workflow_ref` | 呼び出し元のワークフローのファイル（`<オーナー名>/<リポジトリ名>/.github/workflows/<ファイル>@<ref>`）と、ジョブが使ったワークフローのファイル。呼び出し元のリポジトリの名前は `caller.workflow_ref` で読む |
| `caller.actor`、`caller.environment` | ワークフローを起動したアカウントと、ジョブの Environment |
| `request.repositories`、`request.permissions` | 要求したリポジトリと権限。要求の表記と並びのまま記録される |
| `installation.id`、`installation.owner_id` | 取得した Installation の ID と、そのインストール先の Owner ID |
| `revocation` | GitHub が返したトークンを GATES の照合で拒否したときの、そのトークンの失効の結果 |
| `claims_set` | GitHub が署名した OIDC JWT（ワークフローがどのオーナーのどのリポジトリで動いているかを示すトークン）の中身を、署名を確かめた後に写したもの。`caller` にない値（Pull Request の元のブランチ名の `head_ref` など）を含む |

`caller` は、Lambda が呼び出し元を確かめた後にだけ記録されます。
呼び出し元を確かめる前や途中で失敗した要求（`invalid_request`、`invalid_id_token`、`identity_verification_unavailable` など）には `caller` がないため、§2.3 のクエリのうち `caller` で絞るものには現れません。

### 2.3 Common Queries

監査記録は、`aws logs filter-log-events` で、時間の範囲とフィルターを指定して引きます。
`<開始>` と `<終了>` は、ミリ秒の UNIX 時刻です。
フィルターは JSON の行の項目に一致させるため、Lambda が出す `START`、`END`、`REPORT` のような JSON でない行は含まれません。

期間の中で失敗した要求の一覧を、時刻、`request_id`、結果、理由の順に出します。
同じ `result` が続いていれば、[Troubleshooting](troubleshooting.md) の §4 で、そのエラーコードの原因を引きます。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --filter-pattern '{ $.msg = "audit record" && $.result != "success" }' \
  --query 'events[].message' --output json \
  | jq -r '.[] | fromjson | [.time, .request_id, .result, .reason] | @tsv'
```

特定のリポジトリを要求した記録を、時刻、結果、呼び出し元のワークフロー、ワークフロー実行の ID の順に出します。
`<オーナー名>/<リポジトリ名>` には、調べるリポジトリの完全名を入れます。
GATES は名前の大文字と小文字を区別せずに扱い、記録は要求の表記のままのため、どちらも小文字にしてから照らし合わせます。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --filter-pattern '{ $.msg = "audit record" }' \
  --query 'events[].message' --output json \
  | jq -r --arg repo '<オーナー名>/<リポジトリ名>' '.[] | fromjson
      | select((.request.repositories // []) | map(ascii_downcase) | index($repo | ascii_downcase))
      | [.time, .result, .caller.workflow_ref, .caller.run_id] | @tsv'
```

`pull_request_target` と `workflow_run` で起動したジョブに発行した記録を出します。
この 2 つのイベントで起動したジョブは、Pull Request のコードや、ほかのワークフローの成果物を実行するとトークンを渡してしまう危険があります（Security Considerations の §3）。
記録が見つかったら、そのワークフローがトークンを受け取るジョブで信頼できないコードを実行していないかを確かめます。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --filter-pattern '{ $.msg = "audit record" && $.result = "success" && ($.caller.event_name = "pull_request_target" || $.caller.event_name = "workflow_run") }' \
  --query 'events[].message' --output json \
  | jq -r '.[] | fromjson
      | [.time, .caller.event_name, .caller.workflow_ref, .caller.run_id, .claims_set.head_ref // ""] | @tsv'
```

この 2 つのイベントの記録では、`caller.ref` と `caller.sha` は既定のブランチの値になり、起点の Pull Request やワークフロー実行を一つに特定できるとは限りません。
起点は、`caller.run_id` から GitHub のワークフロー実行をたどって調べます（Security Considerations の §6）。

一つの `request_id` か、一つのワークフロー実行の記録を引くクエリは、Troubleshooting の §3 にあります。
本節のクエリは、GATES の検証環境の監査記録で実行し、`request` のない失敗の記録が混ざっても途中で止まらないことを含めて、出力の形を確かめました。

### 2.4 Records Outside GATES

IAM が拒否した呼び出しは Lambda に届かないため、GATES の監査記録には残らず、AWS CloudTrail の記録を見ます（Security Considerations の §6）。
IAM Role の引き受けは、拒否されたものも含めて既定で CloudTrail に残り、時刻で引くクエリは Troubleshooting の §5.1 にあります。
Lambda の呼び出しは、CloudTrail のデータイベントの記録を有効にしたときだけ残ります（[AWS Lambda Invoke Rejection](../research/aws/lambda-invoke-rejection.md)）。
有効にするかどうかは、運用者が自組織の方針で決めます。

## 3. Changing Settings

### 3.1 Applying a Change

`terraform/local/terraform.tfvars` の値を変えたら、次の順に適用します。

1. いまデプロイしているリリースのタグをチェックアウトしてあることを確かめる。別のリリースのソースでは、関数のコードなど、変えた値のほかの差分が Plan に出る
2. `make tf-plan` で Plan を作り、§3.2 から §3.4 に書いた差分だけを示すことを確かめる
3. `make tf-apply` で保存した Plan を適用する
4. もう一度 `make tf-plan` を実行し、`No changes` を示すことを確かめる
5. Deployment の §9.1 のワークフローを実行し、発行が成功することを確かめる

```bash
make tf-plan
make tf-apply
make tf-plan
```

`make tf-apply` が失敗したら、`make tf-plan` から作り直し、Plan を確かめ直してから適用します。
何も適用せずに失敗した場合でも、Terraform が State を書き直すため、保存した Plan が古くなり、次の適用が `Saved plan is stale` で止まることがあります。

`kms_keys` と `active_kms_key` の変更は鍵の更新の一部で、[Key Rotation](rotate-key.md) の手順で行います。
§3.2 から §3.4 の Plan の差分は、GATES の Terraform の定義から見込んだものです。
`terraform.tfvars` の値を変えた Plan は、検証環境で確かめていません。

### 3.2 Audience

`gates_audience` は、GATES 用の OIDC JWT の **Audience**（OIDC JWT の宛先を示す値）です。
GATES は、OIDC JWT の Audience がこの値と完全に一致するときだけ受け付けます。
値の決め方と、変数の検証で拒否される値は、Deployment の §5.3 にあります。

`gates_audience` を変えたら、ワークフローが参照する Variable の `GATES_AUDIENCE` も同じ値に変えます。
Variable を登録した単位（リポジトリごとか Organization か）は、Deployment の §8 のとおりです。
適用と Variable の変更のあいだに動いたワークフローは、Audience が一致せず、`error code "invalid_id_token"` を含む注釈で失敗します。
GATES は Audience を一つしか受け付けないため、この間を作らずに切り替える方法はありません。
ワークフローが動かない時間帯に、適用と Variable の変更を続けて行います。

Plan は、`aws_lambda_function.gates` の環境変数 `GATES_AUDIENCE` の変更だけを示します。

### 3.3 Reserved Concurrency and Log Retention

`lambda_reserved_concurrency` は、Lambda の関数のために予約する同時実行数で、関数が同時に動ける数の上限にもなります。
値の決め方は Deployment の §5.5 にあります。
Plan は、`aws_lambda_function.gates` の `reserved_concurrent_executions` の変更だけを示します。
GATES の検証環境では、AWS の側で予約を 0 にした状態の Plan が、この差分だけを示すことを確かめました（[AWS Fault Injection](../research/aws/fault-injection.md)）。

`log_retention_in_days` は、Lambda のログと監査記録の保持期間（日）です。
受け付ける日数は Deployment の §5.6 にあります。
Plan は、`aws_cloudwatch_log_group.lambda` の `retention_in_days` の変更だけを示します。
保持期間を短くすると、新しい保持期間を過ぎた記録は削除の対象になります。
CloudWatch Logs は、保持期間を過ぎた記録をすぐには消さず、通常 72 時間以内に削除します。
削除された記録は、後から保持期間を延ばしても戻りません。
また、AWS アカウントにアカウント単位の保持のポリシー（account-level retention policy）を設定していると、それがロググループの保持期間より優先されます。

### 3.4 Owner Name after a Rename

許可オーナーの名前が変わったら、`allowed_owner_name` を新しい名前にして適用し直します。
許可オーナーの名前は、ワークフローが引き受ける IAM Role の引き受けの条件（**Trust Policy**）と、Lambda の設定の両方に入っているためです。
適用し直すまで、ワークフローは IAM Role を引き受けられず、Action は `AssumeRoleWithWebIdentity failed` の注釈で失敗します（GATES の設計から導いたもので、検証環境では確かめていません）。

新しい名前は、Deployment の §5.2 のコマンドで、GitHub 上の表記のまま調べます。
Plan は、`aws_iam_role.github_actions` の Trust Policy と、`aws_lambda_function.gates` の環境変数 `GATES_ALLOWED_OWNER_NAME` の変更だけを示します。
ワークフロー利用者の `repositories` の入力に古い名前が残っていれば、それも直してもらいます。

### 3.5 Values Fixed at Deployment

次の値は、デプロイのときに決め、変えないものとして扱います。
変える手順は定めておらず、検証環境でも確かめていません。

- **`allowed_owner_id` と `allowed_owner_type`**：許可オーナーそのものを表す。別のアカウントに変えると、GitHub App のインストール先と、接続情報を登録したリポジトリも変わる
- **`github_app_client_id`**：GATES がトークンを発行する GitHub App を表す。KMS Key にインポートした秘密鍵は元の GitHub App のもののため、Client ID だけを別の GitHub App に変えても、その GitHub App は GATES の署名を受け付けない

## 4. Updating GATES

### 4.1 Order of the Update

GATES を新しいリリースに更新するときは、Lambda の関数のコードと、ワークフローが参照する Action の両方を、同じリリースにそろえます。
Action と Lambda のあいだの要求と応答の形を、リリースをまたいで保つ規則はなく、異なるリリースの組み合わせで動くかは確かめていないためです。
組み合わせが合わないと、Action の要求を Lambda が拒否する（`invalid_request`）か、Action が Lambda の応答を読めない（`the Lambda response` か `the success response has no usable` で始まる注釈）見込みです（確かめていません）。

更新は、Lambda を先に、Action を後に行います。
Lambda は運用者が一度の適用で切り替えられますが、Action の参照先は、ワークフロー利用者が各ワークフローで書き換えるため、一斉には切り替わらないからです。
Lambda を更新してから、すべてのワークフローの参照先が更新されるまでのあいだは、異なるリリースの組み合わせが動きます。
この期間を短くするため、更新の前にワークフロー利用者に更新の時期を伝えておきます。

本節の手順は、検証環境で新しいリリースへの更新として通しては確かめていません。
手順の各操作（配布物の検証、`make tf-plan` と `make tf-apply`、関数のコードの照合）は、Deployment で確かめた操作と同じです。

### 4.2 Before the Update

リリースのページ（`https://github.com/tmknom/gates/releases`）で、更新先のリリースの変更点を読みます。
とくに、Terraform の変数の追加や変更があるかを確かめます。
変数の差分は、更新前と更新後のタグのあいだの `terraform/terraform.tfvars.example` の差分でも確かめられます。
`<更新前のバージョン>` と `<更新後のバージョン>` には、`vX.Y.Z` の形のリリースのタグを入れます。

```bash
git fetch --tags
git diff <更新前のバージョン> <更新後のバージョン> -- terraform/terraform.tfvars.example
```

変数が加わっていれば、`terraform/local/terraform.tfvars` に値を足します。

更新先のリリースの配布物（Lambda の関数のコードの zip）をダウンロードし、[Release Verification](verify-release.md) の §3 と §4 の手順で、チェックサムと **Attestation**（配布物をどのリポジトリのどのコミットから、どのワークフローでビルドしたかを示す署名付きの記録）を検証します。
検証に失敗した配布物のリリースには、更新しません。

### 4.3 Applying the New Release

更新先のリリースのタグをチェックアウトします。

```bash
git switch --detach <更新後のバージョン>
```

`make tf-plan` を実行し、Plan を確かめます。
変数の追加がなく、Terraform の定義が変わっていないリリースなら、Plan は `aws_lambda_function.gates` の `source_code_hash` の変更（関数のコードの更新）だけを示します。
リリースの変更点に書かれていない差分が現れたら、適用しません。

```bash
make tf-plan
```

`make tf-plan` が `Instance cannot be destroyed` のエラーで止まったら、Plan が KMS Key の削除か置換を含んでいます。
KMS Key には、Terraform による削除と置換を止める設定（`prevent_destroy`）を付けてあるためです。
このとき `make tf-plan` は保存した Plan を捨てるため、`make tf-apply` は実行できません。
出力の中で、KMS Key（`aws_kms_external_key.signing`）の属性に `# forces replacement` が付いた行を探し、どの値の変更が置換を起こしたかを調べます。
たとえば、`kms_keys` の既存の鍵の `key_spec` を変えると、作成の後に変えられない属性のため置換になります。
`prevent_destroy` を外して適用することはしません。
KMS Key を削除すると、インポートした秘密鍵も失われ、GATES はその鍵で署名できなくなるためです。

Plan に問題がなければ、適用し、適用の後の `make tf-plan` が `No changes` を示すことを確かめます。

```bash
make tf-apply
make tf-plan
```

### 4.4 Checking the Updated Deployment

デプロイした関数のコードが、§4.2 で検証した配布物と同じであることを確かめます。
コマンドと、一致しないときの切り分けは Deployment の §6.4 と同じです。

```bash
aws lambda get-function --function-name gates --query Configuration.CodeSha256 --output text
openssl dgst -sha256 -binary <配布物のパス> | openssl base64
```

続けて、Deployment の §9.1 のワークフローで、Action の参照先を更新先のタグのコミットの SHA（`git rev-parse HEAD` の出力）に変えて実行し、発行が成功することを確かめます。
成功したら、ワークフロー利用者に、Action の参照先を同じコミットに更新するよう伝えます。
Action はタグではなくコミットの SHA で参照するため、ワークフロー利用者が参照先を書き換えるまで、ワークフローは更新前の Action を使い続けます。

### 4.5 Returning to the Previous Release

更新の後に問題が見つかったら、更新前のリリースのタグをチェックアウトし、§4.3 と §4.4 と同じ手順で適用し直します。
関数のコードの照合には、更新前のリリースの配布物を使います。
ワークフロー利用者が Action の参照先を更新済みなら、更新前のコミットに戻してもらいます。

更新先のリリースで Terraform の定義が変わっていると、更新前のタグの Plan に、関数のコードのほかの差分が出ることがあります。
その場合は差分の内容を確かめ、KMS Key の削除や置換が含まれていれば適用しません。
更新前のリリースに戻す手順は、検証環境で確かめていません。

## 5. When to Rotate the Key

GATES は、秘密鍵を KMS Key にインポートして使います。
KMS Key にインポートした秘密鍵の中身（**鍵素材**）は、別の鍵素材に入れ替えられません。
このため、秘密鍵を替えるときは、新しい KMS Key を加えて新しい秘密鍵をインポートし、Lambda が署名に使う鍵（**使用中の鍵**）を切り替える鍵の更新を行います。
手順は [Key Rotation](rotate-key.md) にあり、発行を止めずに進められます。

次のときに、鍵の更新を行います。

- **定期の更新**：GitHub App の秘密鍵には有効期限がなく、削除するまで使える。更新の間隔は GATES では定めず、自組織の鍵の管理の方針に合わせる
- **秘密鍵のコピーが残った疑い**：インポートのときに手元に置いた秘密鍵のファイルが、削除されずに残ったか、バックアップなどに写った疑いがあるとき。秘密鍵を持つ人は、GATES を経ずに GitHub App としてトークンを作れる
- **鍵を扱う権限を持つ人の交代**：Terraform を適用する権限、Key Policy を書き換える権限、鍵素材をインポートする権限を持っていた人が外れたとき。鍵素材をインポートした人は、秘密鍵を手元に持っていた。Key Policy を書き換えられた人（Terraform を適用できる人を含む）は、KMS Key に自分の署名を許可する **Grant**（Key Policy とは別に KMS Key の操作を許可する仕組み）を作れた。Grant は Key Policy を元に戻しても残る（Security Considerations の §4）。新しい KMS Key には、それまでに作られた Grant はない

外れた人の AWS の権限を取り除くことは、鍵の更新とは別に行います。
KMS Key で署名する能力の悪用がすでに疑われるときは、鍵の更新の前に、§6 で署名を止めます。

## 6. Stopping GATES When Abuse Is Suspected

### 6.1 Choosing How to Stop

悪用が疑われるときに止めたいものは、次に分かれます。

- **GATES の発行**：許可オーナーのワークフローが、GATES を通して不正にトークンを受け取っている疑い。たとえば、`pull_request_target` のジョブが Pull Request のコードを実行している（§2.3）
- **GitHub App としての署名**：KMS Key で署名する能力が、GATES を経ずに使われている疑い。AWS アカウントで Key Policy、Lambda、Lambda の **実行 Role**（Lambda が KMS での署名とログの書き込みに使う IAM Role）を書き換えられる主体は、GATES の認可を経ずに GitHub App としてトークンを作れる（Security Considerations の §4）

止める手段を、止められる範囲と、戻し方で比べると次のとおりです。

| 手段 | 止められる範囲 | 戻せるか | 戻せる主体 |
|---|---|---|---|
| Reserved Concurrency を 0 にする（§6.2） | GATES の発行だけ。KMS Key での署名は止まらない | 戻せる | Lambda の関数の設定を変えられる主体 |
| KMS Key を無効にする（§6.3） | 使用中の鍵での署名。GATES の発行も止まる | 戻せる | `kms:EnableKey` を持つ主体 |
| Installation を停止する（§6.4） | GitHub App による許可オーナーのリソースへのアクセス。GATES の発行も止まる | 戻せる | 停止した側だけ |
| GitHub で秘密鍵を削除する（§6.5） | 削除した鍵での署名による認証。GATES の発行も止まる | 戻せない。新しい鍵で鍵の更新を行う | なし |

署名の能力の悪用を、AWS の側の権限に左右されずに止められる見込みがあるのは、GitHub での秘密鍵の削除だけです（§6.5）。
KMS Key の無効化は、`kms:EnableKey` を持つ主体が戻せるためです。
Installation の停止は、AWS の権限では解けない点で秘密鍵の削除に近いものの、GATES を経ずに作った GitHub App JWT でのトークンの発行を止められるかは確かめていません（§6.4）。
KMS Key の無効化と秘密鍵の削除を比べた根拠は、[Security Design](../design/security.md) の §5.1 にあります。
署名の悪用を疑うときは、まず §6.3 で署名を止め、§6.5 で秘密鍵を削除します。

どの手段でも、止める前に発行されたトークンは、有効期限（発行から 1 時間）まで使えるまま残り得ます。
GATES が発行したトークンの対象と権限は、要求したリポジトリと `contents:read` に限られます。
GATES を経ずに作られたトークンの上限は、GitHub App の権限とインストール対象です。

止めた後の調査には、§2.3 の監査記録と、AWS CloudTrail の記録を使います。
KMS Key の署名の記録（`Sign`）は、署名した主体を `userIdentity.arn` に持ちます。
次のコマンドで、期間の中の署名を主体ごとに数えます。
`<開始>` と `<終了>` は、`YYYY-MM-DDThh:mm:ssZ` の形の ISO 8601 の時刻です。

```bash
aws cloudtrail lookup-events --region <KMS Key のリージョン> \
  --lookup-attributes AttributeKey=EventName,AttributeValue=Sign \
  --start-time <開始> --end-time <終了> \
  --query 'Events[].CloudTrailEvent' --output json \
  | jq -r '.[] | fromjson | [.userIdentity.arn, .resources[0].ARN] | @tsv' | sort | uniq -c
```

GATES の検証環境では、GATES の Lambda の署名の主体は `arn:aws:sts::<AWS アカウント ID>:assumed-role/gates-lambda/<セッション名>` でした。
これと違う主体の署名があれば、GATES を経ない署名の疑いがあります。
GATES を経ない署名の記録は、検証環境では観測していません。

### 6.2 Setting Reserved Concurrency to 0

Lambda の関数の Reserved Concurrency を 0 にすると、関数のすべての呼び出しが拒否されます。
許可オーナーのすべてのワークフローで GATES の発行が止まり、ほかの手段より影響の範囲が分かりやすいため、GATES の発行の悪用を疑うときに最初に取れる手段です。

```bash
aws lambda put-function-concurrency --function-name gates --reserved-concurrent-executions 0
```

GATES の検証環境では、この後の要求は関数に届かず、Action は `the Lambda Invoke failed: AWS error code "TooManyRequestsException", HTTP status 429` の注釈で失敗しました。
監査記録は残らず、関数のロググループにも行は残りませんでした（AWS Fault Injection）。

止めているあいだの `make tf-plan` は、`reserved_concurrent_executions` を `terraform.tfvars` の値に戻す差分を示します（確かめたのは、`terraform.tfvars` の値が `10` の場合です）。
止めているあいだにほかの理由で適用すると、発行が再開します。
長く止めるときは、`terraform.tfvars` の `lambda_reserved_concurrency` も `0` にして、§3.1 の手順で適用します（手順 5 の発行の確認は行わない）。

戻すときは、`terraform.tfvars` の `lambda_reserved_concurrency` を `0` にしていれば元の値に戻し、§3.1 の手順で適用します。
検証環境では、`terraform apply` で予約を元の値に戻した後の要求が成功しました。
`lambda_reserved_concurrency` が既定の `-1`（予約しない）だった場合の戻し方は、確かめていません。

### 6.3 Disabling the KMS Key

使用中の鍵を無効にすると、Lambda は GitHub App JWT（GATES が GitHub App として GitHub API を呼ぶための、KMS Key で署名したトークン）に署名できなくなります。
同じ KMS Key で署名しようとするほかの主体も、署名できなくなります。
使用中の鍵の ARN は、関数の設定から読みます。

```bash
aws lambda get-function --function-name gates \
  --query 'Configuration.Environment.Variables.GATES_KMS_KEY_ARN' --output text
aws kms disable-key --key-id <KMS Key の ARN>
aws kms describe-key --key-id <KMS Key の ARN> --query 'KeyMetadata.KeyState' --output text
```

状態が `Disabled` であることを確かめます。
GATES の検証環境では、無効にしてから約 1 分後の要求で、Action は `error code "internal_error"` を含む注釈で失敗しました。
監査記録の `reason` は `kms sign failed: code "DisabledException"` を含みました（AWS Fault Injection）。

KMS Key の無効化は Terraform の Plan に現れないため、`make tf-apply` を実行しても無効のままです。
戻すときは、`aws kms enable-key --key-id <KMS Key の ARN>` で有効にします。
検証環境では、有効に戻した後の要求が成功しました。
KMS Key を無効にする前に署名された GitHub App JWT は、無効にした後も、その有効期限まで GitHub に受け付けられ得ます。
有効期限は、GATES の Lambda が署名したものは 5 分、GATES を経ずに署名されたものは GitHub の上限の 10 分までです（[GitHub App JWT](../design/github-app-jwt.md)）。

### 6.4 Suspending the Installation

Installation を停止すると、GitHub App は、許可オーナーのリソースにアクセスできなくなります（GitHub の公式資料による）。
§1.2 の Installation の設定画面で `Suspend` をクリックします。

GATES の検証環境では、停止の後の要求で、GATES は Installation が停止していると判定して発行を止めました。
監査記録の `reason` は `installation is suspended` を含みました（GitHub App Installation Changes、Troubleshooting の §4）。
現在のリリースでは、この失敗は `installation_unavailable` のエラーコードになり、Action はそれを含む注釈で失敗します。
GATES を経ずに作った GitHub App JWT で、停止中の Installation のトークンを発行できないかと、停止の前に発行されたトークンが使えなくなるかは、確かめていません。

停止を解けるのは、停止した側だけです。
Installation の設定画面で停止した場合は、同じ画面の `Unsuspend` で解きます。
GitHub App を作った側（運用者）が GitHub API で停止した場合は、許可オーナーの側からは解けません。
検証環境では、停止を解いた後の要求が、GATES の設定を変えずに成功しました。

### 6.5 Deleting the Private Key from GitHub

GitHub App の設定ページの `Credentials` → `Key pairs` で、使用中の鍵に対応する秘密鍵を削除すると、その鍵で署名した GitHub App JWT は、GitHub App としての認証に使えなくなる見込みです。
これは GitHub の公式資料（秘密鍵は削除するまで有効で、漏えいした鍵は削除して取り除く）からの推論で、削除した鍵で署名した JWT を GitHub が拒否することは、GATES を通しては確かめられません。
削除した秘密鍵は戻せないため、GATES の発行を再開するには、新しい秘密鍵で鍵の更新を行います。

使用中の鍵に対応する秘密鍵は、公開鍵のフィンガープリントで見分けます。
次のコマンドの出力と、`Key pairs` に表示される鍵の `SHA256:` の後の値を比べます。

```bash
aws kms get-public-key --key-id <KMS Key の ARN> --query PublicKey --output text \
  | openssl base64 -d -A | openssl sha256 -binary | openssl base64
```

GitHub は GitHub App の最後の秘密鍵を削除させないため、`Key pairs` に鍵が一つしかなければ、先に `Key pairs` の `New key` で新しい鍵を生成します。
生成した鍵は、ブラウザーがファイルとしてダウンロードします。
すぐに鍵の更新でインポートしないなら、ダウンロードしたファイルを削除します。
後で鍵の更新を行うときは、Key Rotation の §4 で改めて鍵を生成し、ここで生成して使わなかった鍵も `Key pairs` から削除します。

削除の後、GATES の発行は失敗します。
Action の注釈には `error code "installation_unavailable"` が出る見込みです（確かめていません）。

発行を再開する前に、悪用した主体の AWS の権限を取り除きます。
権限が残ったまま鍵の更新を行うと、新しい KMS Key での署名も同じ主体に悪用され得るためです。

§6.3 か §6.5 で止めた後に Key Rotation の手順で発行を再開するときは、次のように読み替えます。
止めた後の状態から鍵を更新する手順は、検証環境で確かめていません。

- Key Rotation の §2 の前提のうち、ワークフローが成功することは満たせない。Key Rotation の §5 で使用中の鍵を新しい鍵に切り替えるまで、発行は失敗したままになる
- 旧鍵に切り戻す手順（Key Rotation の §6 と §7 の切り戻し）は使わない
- 旧 KMS Key の無効化（Key Rotation の §7）と、GitHub からの旧秘密鍵の削除（Key Rotation の §8）は、済んでいれば省く

## 7. References

- [Deployment](deployment.md)
- [Release Verification](verify-release.md)
- [Key Rotation](rotate-key.md)
- [Troubleshooting](troubleshooting.md)
- [Security Considerations](security-considerations.md)
- [Lambda Design](../design/lambda.md)
- [Security Design](../design/security.md)
- [GitHub App JWT](../design/github-app-jwt.md)
- [GitHub App Installation Changes](../research/github/installation-changes.md)
- [AWS Fault Injection](../research/aws/fault-injection.md)
- [AWS Lambda Invoke Rejection](../research/aws/lambda-invoke-rejection.md)
- [GitHub — Reviewing and modifying installed GitHub Apps](https://docs.github.com/en/apps/using-github-apps/reviewing-and-modifying-installed-github-apps)
- [GitHub — Suspending a GitHub App installation](https://docs.github.com/en/apps/maintaining-github-apps/suspending-a-github-app-installation)
- [GitHub — Managing private keys for GitHub Apps](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps)
- [AWS — Change log data retention in CloudWatch Logs](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/Working-with-log-groups-and-streams.html)
- [AWS — Filter pattern syntax for metric filters, subscription filters, filter log events, and Live Tail](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/FilterAndPatternSyntax.html)
