# Troubleshooting

Action が失敗したときに、注釈から原因と直し方を引くためのリファレンスです。
ワークフロー利用者は §1 と §2 を、運用者は §3 以降を使います。

Action が失敗すると、ジョブのログとワークフロー実行のページに `::error::` の注釈が出ます。
注釈の先頭の文言が、失敗した段階を示します。

## 1. Failures the Workflow User Can Fix

| 注釈 | 原因 | 直し方 |
|---|---|---|
| `ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set` で始まる | ジョブに `id-token: write` がない。ジョブに `permissions` を書くと、ワークフローの `permissions` は使われない | Action を置いたジョブの `permissions` に `id-token: write` を書く |
| `input <入力名> is empty` で始まる | 入力が空。参照した Secret か Variable がないか、参照できない | 入力名と、Secret と Variable の名前の綴りを見直す。正しければ、運用者に登録を確かめてもらう |
| `invalid inputs` で始まる | `repositories` か `permissions` の書式の誤り | 注釈の次の行に出る要素と理由に従って直す |
| `could not get the OIDC JWT for GATES` で始まり、`HTTP status 400` を含む | `audience` が、GitHub が受け付けない値 | `audience` に、運用者が登録した Variable を渡す |
| `could not get the OIDC JWT for` で始まり、`HTTP status 400` を含まない | GitHub の一時的な障害か、Runner から OIDC JWT の取得先に届かない | 再実行する。セルフホストの Runner なら、通信の許可を確かめる |
| `AssumeRoleWithWebIdentity failed` で始まる | `role-arn` の誤り、ワークフローを置くリポジトリが許可オーナーのものでない、またはリポジトリの OIDC JWT が不変の ID を含む形式でない | `role-arn` が `arn:aws:iam::${{ secrets.GATES_AWS_ACCOUNT_ID }}:role/${{ vars.GATES_ROLE_NAME }}` の形かを見直す。正しければ §2 に進む |
| `AssumeRoleWithWebIdentity failed` で始まり、Dependabot のワークフローでだけ起きる | Dependabot のワークフローは GitHub Actions の Secrets を参照できない | 運用者に、Dependabot の Secrets への登録を頼む |
| `the Lambda Invoke failed` で始まり、`AccessDeniedException` を含む | `function-name` か `aws-region` の誤り | 運用者が登録した Variables を渡す |
| `error code "invalid_request"` を含む | 要求の形が正しくない（名前に使えない文字、500 を超える `repositories`、大文字の権限名など） | `repositories` は `owner/repo` の形で書く。`permissions` は `contents:read` のように小文字の `権限名:水準` で書く |
| `error code "invalid_id_token"` を含む | `audience` が GATES の設定と一致しない | 運用者が登録した Variable を渡す。渡しているなら §2 に進む |
| `error code "authorization_denied"` を含む | 許可オーナー以外のリポジトリを要求した。[Action Reference](action-reference.md) の「要求できる権限」にない権限名か水準（Organization permissions の `members` など）か、発行上限が許さない権限を要求した。または、保護対象を要求したが、ワークフローのリポジトリがその許可 Caller でないか、権限の上限が許さない権限を要求した。リポジトリポリシーを適用するデプロイでは、要求したリポジトリについて、リポジトリポリシーがないか誤りがある、ワークフローのリポジトリを許していない、権限の上限が許さない権限を要求した、のどれかに当たる。または、11 件以上のリポジトリを要求した | 許可オーナーのリポジトリと、運用者から受け取った使える権限だけを要求する。保護対象は、許可 Caller のリポジトリから、権限の上限の範囲で要求する。リポジトリポリシーを適用するデプロイでは、11 件以上なら 10 件以下に分ける。10 件以下でも拒否されるなら、リポジトリを 1 件ずつ要求して拒否されるリポジトリを見つけ、そのリポジトリの管理者にリポジトリポリシーを確かめてもらう。一つでも許可されないと、要求全体が失敗する。注釈からは原因を見分けられないため、直らなければ §2 に進む。運用者は `request_id` で原因を調べられる |
| `error code "github_issuance_rejected"` を含む | インストール対象にないか、存在しないリポジトリを要求した。または、GitHub App に付与されていない権限か、付与された水準より高い水準を要求した | リポジトリの綴りを確かめる。正しければ、インストール対象に加えるよう頼むか、要求から外す。権限は、運用者から受け取った使える権限と水準に直す。どちらも正しいなら §2 に進む |
| ジョブの `outputs` に置いたトークンが空になる | GitHub はマスクされた値をジョブの外に渡さない | トークンが要るジョブごとに Action のステップを置く |
| トークンで呼んだ API が `403` を返す | トークンに、その操作に要る権限がない（要求しなかった権限か、`read` で要求した権限の書き込み）。または、インストール対象外の公開リポジトリへの書き込み | 権限の不足なら、応答の `X-Accepted-GitHub-Permissions` ヘッダーが要る権限を `権限名=水準` の形で示す（`;` で区切った候補はどれか一つを満たせばよく、候補の中で `,` でつないだ権限はすべて要る）。`権限名:水準` に直して `permissions` に加える。運用者から受け取った使える権限になければ、運用者に相談する。インストール対象外のリポジトリなら、インストール対象に加えるよう頼む |
| `git push` が `refusing to allow a GitHub App to create or update workflow` で拒否される | `.github/workflows/` のファイルを変える push に、`workflows:write` がない | 使える権限に `workflows:write` があれば加える。なければ、ワークフローのファイルを変えない |
| トークンで呼んだ API が `404` を返す | 要求に含めなかったプライベートリポジトリ（インストール対象外のものを含む）。または、リポジトリの設定でその機能が無効（Pull requests の機能が無効なら、`pull_requests` の権限があっても Pull Request の API は `404`） | インストール対象のリポジトリなら `repositories` に加える。対象外なら、インストール対象に加えるよう頼み、加わったら `repositories` に加える。機能が無効なら、リポジトリの管理者に有効にしてもらう |

## 2. Reporting a Failure to the Operator

§1 に当たらない失敗と、§1 の直し方で解消しない失敗は、運用者に伝えます。
ただし、注釈の `message` が `Retry later.` で終わる失敗は一時的な障害のため、まず再実行します。
`the job was canceled` は、ジョブがキャンセルされただけです。

運用者には、次を伝えます。
注釈はトークンも認証情報も含まないため、そのまま渡せます。

- 注釈の全文
- 注釈にあれば、`request_id`
- ワークフロー実行の URL と、何回目の実行か
- 失敗した時刻（ログの `Invoking the Lambda function at` の行の時刻。なければ注釈の時刻）

## 3. Finding the Audit Record

ここからは運用者の作業です。
コマンドは、GATES を適用した AWS アカウントのログと設定を読める一時認証情報で、`AWS_REGION` を GATES のリージョンにして実行します。

`request_id` があれば、それで監査記録を引きます。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --filter-pattern '{ $.request_id = "<request_id>" }' \
  --query 'events[].message' --output json \
  | jq '.[] | fromjson | select(.msg == "audit record") | del(.claims_set)'
```

`result` がエラーコードで、`reason` が失敗した理由です。
`reason` は応答にも注釈にも出ないため、ワークフロー利用者には見えません。

`request_id` がなく、注釈が `GATES returned a failure response` で始まるときは、ワークフロー実行の ID（URL の `.../actions/runs/<ID>`）で引きます。
呼び出し元を確かめる前に失敗した要求（`invalid_request`、`invalid_id_token` など）は、この方法では見つからないため、時刻で引きます。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --filter-pattern '{ $.caller.run_id = <ワークフロー実行の ID> }' \
  --query 'events[].message' --output json \
  | jq -c '.[] | fromjson | {time, request_id, result, reason, run_attempt: .caller.run_attempt}'
```

注釈に `request_id` がない失敗は §5 で調べます。

## 4. Causes and Fixes by Error Code

| エラーコード | 原因 | 確かめることと直し方 |
|---|---|---|
| `invalid_request` | 要求の形が正しくない。Action と GATES のリリースが違う場合にも起きる | `reason` で不正なフィールドを見る。ワークフロー利用者に §1 の直し方を伝える。Action の参照先がデプロイした GATES と同じリリースかを確かめる |
| `invalid_id_token` | GATES 用の OIDC JWT が受け付けられない。Audience の食い違い、期限切れ、署名の不正など | Variable の `GATES_AUDIENCE` が `gates_audience` と同じかを確かめる |
| `authorization_denied` | 許可オーナー以外のリポジトリか、要求できない権限の要求。または、保護対象を含み、Caller が許可 Caller に当たらないか、権限の上限を超える要求。リポジトリポリシーを適用するデプロイでは、リポジトリポリシーが許さない要求と、11 件以上のリポジトリの要求。原因は `reason` で見分ける | 「Reasons for authorization_denied」の表に従う |
| `identity_verification_unavailable` | GitHub の OIDC の公開鍵を一時的に取得できない | 再実行で解消し得る。繰り返すなら [GitHub Status](https://www.githubstatus.com/) を見る |
| `signing_unavailable` | AWS KMS の署名が一時的に失敗した | 再実行で解消し得る。`reason` が `ThrottlingException` で繰り返すなら、KMS のクォータを確かめる |
| `installation_unavailable` | GATES が Installation を使えない。GitHub App のアンインストール（`status 404`）、Installation の停止（`installation is suspended`）、`github_app_client_id` の誤り、使用中の鍵と GitHub App の鍵の不一致、GitHub からの鍵の削除 | アンインストールなら、`Only select repositories` で再インストールする。停止なら、悪用への対処で意図して停止したものでないかを確かめてから解く。鍵の不一致は下記で確かめる |
| `github_api_unavailable` | GitHub API が一時的に失敗した。リポジトリポリシーを適用するデプロイでは、リポジトリポリシーを読むトークンの発行か、リポジトリポリシーの読み取りが一時的に失敗した場合と、すべてのリポジトリのリポジトリポリシーの読み取りが時間内に終わらなかった場合も含む（`reason` が `issue policy read token: ` か `fetch repository policy: ` で始まる） | 再実行で解消し得る。繰り返すなら GitHub Status を見る |
| `github_issuance_rejected` | インストール対象にないか、存在しないリポジトリの要求。または、GitHub App に付与されていない権限か、付与された水準より高い水準の要求。リポジトリポリシーを適用するデプロイで、GitHub App に `Single file` の権限が付与されていないか、Installation で承認されていない場合も、認可を通った要求はどれも `github_issuance_rejected` になる（`reason` が `issue policy read token: ` で始まる） | `request.repositories` を Installation の画面のリポジトリと比べる。`request.permissions` を GitHub App の Repository permissions と比べる。`reason` が `issue policy read token: ` で始まり、リポジトリが正しいなら、GitHub App の `Single file` の付与と承認を確かめる。どれも正しいのに拒否されるなら、`reason` の GitHub の request ID を控えて GitHub に問い合わせる |
| `issuance_verification_failed` | GitHub が返したトークンが要求と合わない。`reason` が `protected repositories[<J>]` を含むなら、保護対象の名前と Repository ID の組が保護対象の一覧と食い違っている。`reason` が `single_file_paths` を含むなら、GitHub App の `Single file` のパスが `.github/gates-policy.yml` だけでない。`does not match the repository whose repository policy was read` を含むなら、リポジトリポリシーを読んだ直後に、そのリポジトリが作り直されたか、別のリポジトリがその名前になった | `protected repositories[<J>]` を含むなら、「Protected Repository Mismatch」で保護対象の一覧を直す。`single_file_paths` を含むなら、GitHub App の `Single file` のパスを `.github/gates-policy.yml` だけにする。リポジトリポリシーを読んだリポジトリとの食い違いは、再実行で解消し得る。どれでもないなら、`reason` と GitHub の request ID を控え、GATES の開発者に報告する。どの場合も、`revocation` が `failed` なら、トークンは期限（発行から 1 時間）まで残り得る。`reason` が `issue policy read token: ` で始まる照合失敗では、リポジトリポリシーを読むトークンの失効の結果が `policy_read_revocation` にあり、`failed` なら同じく残り得る |
| `internal_error` | ほかに当たらない失敗。使用中の KMS Key が使えない（無効、削除の予約、鍵素材のインポート前）、Lambda の権限の不足、Lambda の panic など。リポジトリポリシーを適用するデプロイでは、リポジトリポリシーの読み取りに対して GitHub がファイルの有無を示さない応答を返した場合と、`.` か `..` という名前のリポジトリを要求した場合も含む（`reason` が `fetch repository policy: ` で始まる） | `reason` が `fetch repository policy: ` で始まるなら、`.` か `..` の名前の要求はワークフロー利用者に直してもらい、そうでなければ `reason` を控えて GATES の開発者に報告する。ほかは、下記で KMS Key の状態を確かめる。`Disabled` なら、悪用への対処や鍵の更新で意図して無効にしたものでないかを確かめてから有効にする。`PendingDeletion` なら削除の予約を取り消す。`PendingImport` なら [Key Material Import](import-key-material.md) で鍵素材をインポートする。panic なら、同じ `request_id` の `panic recovered` の行を見る |

GitHub App に権限を加えた後に `github_issuance_rejected` か `issuance_verification_failed` が続くなら、Installation の設定画面に `Permission updates requested. Review request.` が出ていないかを確かめます。
GitHub App に加えた権限は、Installation の側で承認するまで Installation に付与されません。

使用中の KMS Key の状態を確かめます。

```bash
aws lambda get-function --function-name gates \
  --query 'Configuration.Environment.Variables.GATES_KMS_KEY_ARN' --output text
aws kms describe-key --key-id <KMS Key の ARN> --query 'KeyMetadata.KeyState' --output text
```

使用中の鍵と GitHub App の鍵が一致するかは、次の出力と、GitHub App の `Credentials` → `Key pairs` の `SHA256:` の後の値を比べて確かめます。
一致する鍵がなければ、鍵の更新で新しい鍵に替えます。

```bash
aws kms get-public-key --key-id <KMS Key の ARN> --query PublicKey --output text \
  | openssl base64 -d -A | openssl sha256 -binary | openssl base64
```

### 4.1 Reasons for authorization_denied

`authorization_denied` の `reason` は `authorization denied: ` で始まり、続く文言が満たさなかった条件を示します。
保護対象の条件で拒否したときは、続く文言が `repositories[<I>] matches protected repositories[<J>]: ` で始まります。
`<I>` は `request.repositories` の、`<J>` は `protected_repositories` の `repositories` の、0 から数えた位置です。
`callers[<K>]` は、その保護対象の `callers` の位置です。
`reason` には Repository ID も名前も出ないため、位置で保護対象の一覧の要素を探します。
Caller のリポジトリの完全名は `claims_set.repository` に、Repository ID は `caller.repository_id` にあります。
表の `...` は、`repositories[<I>] matches protected repositories[<J>]` です。

| `authorization denied: ` に続く文言 | 原因 | 直し方 |
|---|---|---|
| `repositories[<I>] owner name does not match allowed owner name` | 許可オーナー以外のリポジトリを要求した | ワークフロー利用者に、許可オーナーのリポジトリだけを要求してもらう |
| `permissions are not within allowed permissions` | Action Reference の「要求できる権限」にない権限名か水準を要求した | ワークフロー利用者に §1 の直し方を伝える |
| `permissions are not within issuable permissions` | 発行上限が許さない権限名か水準を要求した（発行上限に `read` だけを含めた権限名の `write` など） | ワークフローに必要なら、発行上限と GitHub App の権限の両方に加えるかを判断する。加えた権限は、保護対象でないリポジトリに対して、許可オーナーのすべてのワークフローが要求できるようになる |
| `...: no allowed callers are listed` | 許可 Caller のない保護対象を要求した | 要求から外してもらう。許可するなら、悪用への対処で意図して止めたものでないかを確かめてから、許可 Caller を加える |
| `...: no allowed caller matches both the caller repository id and name` | Caller が、その保護対象の許可 Caller でない | 許可するかを判断する。許可するなら、Caller のリポジトリを許可 Caller として保護対象の一覧に加える |
| `...: callers[<K>] matches the caller repository id but not the name` | 許可 Caller の Repository ID は Caller と一致し、名前が一致しない。許可 Caller のリポジトリを改名したか、一覧の名前を書き誤った | 許可 Caller の `repository_name` を、`claims_set.repository` のリポジトリ名に直す |
| `...: callers[<K>] matches the caller repository name but not the id` | 許可 Caller の名前は Caller と一致し、Repository ID が一致しない。許可 Caller のリポジトリを削除して作り直したか、一覧の ID を書き誤った | Caller のリポジトリが許可してよいものかを確かめ、許可 Caller の `repository_id` を `caller.repository_id` の値に直す |
| `...: permissions are not within the permission limit of callers[<K>]` | 許可 Caller の権限の上限が許さない権限名か水準を要求した | ワークフロー利用者に要求を直してもらうか、権限の上限を広げるかを判断する |

一覧を直したら、`make tf-plan` で差分を確かめてから適用します。

リポジトリポリシーを適用するデプロイでは、次の文言も現れます。
リポジトリポリシーで拒否したときは、続く文言が `repositories[<I>]: ` で始まります。
`reason` にはリポジトリポリシーの内容も行の位置も出ません。
運用者は、`request.repositories` の `<I>` 番目のリポジトリの管理者に、そのリポジトリと、Caller のリポジトリ（`claims_set.repository`）と Repository ID（`caller.repository_id`）を伝え、ファイルを [Repository Policy](repository-policy.md) に照らして確かめてもらいます。

| `authorization denied: ` に続く文言 | 原因 | 直し方 |
|---|---|---|
| `repositories count exceeds the limit under repository policy enforcement` | 11 件以上のリポジトリを要求した | ワークフロー利用者に、要求を 10 件以下に分けてもらう |
| `repositories[<I>]: repository policy is not found` | リポジトリポリシーがない | リポジトリの管理者に、既定ブランチの `.github/gates-policy.yml` に置いてもらう |
| `repositories[<I>]: repository policy is not a file` | パスにあるのがディレクトリか、存在しない先へのシンボリックリンク | リポジトリの管理者に、ファイルとして置き直してもらう |
| `repositories[<I>]: repository policy is too large` | リポジトリポリシーが 64 KiB を超える | リポジトリの管理者に、許可 Caller を減らしてもらう |
| `repositories[<I>]: repository policy is invalid` | 書式か内容に誤りがある。どこが誤りかは記録されない | リポジトリの管理者に、ファイルを Repository Policy に照らして直してもらう |
| `repositories[<I>]: repository policy lists no allowed callers` | `callers: []` で、どの Caller も許していない | リポジトリの管理者に、許すかを判断してもらう |
| `repositories[<I>]: repository policy has no allowed caller matching both the caller repository id and name` | Caller を許していない | リポジトリの管理者に、許すかを判断してもらう |
| `repositories[<I>]: repository policy has an allowed caller matching the caller repository id but not the name` | 許可 Caller の Repository ID は一致し、名前が一致しない。Caller のリポジトリを改名したか、名前を書き誤った | リポジトリの管理者に、`repository_name` を `claims_set.repository` のリポジトリ名に直してもらう |
| `repositories[<I>]: repository policy has an allowed caller matching the caller repository name but not the id` | 許可 Caller の名前は一致し、Repository ID が一致しない。Caller のリポジトリを削除して作り直したか、ID を書き誤った | リポジトリの管理者に、`caller.repository_id` のリポジトリを許してよいかを確かめてもらい、許すなら `repository_id` をその値に直してもらう |
| `repositories[<I>]: permissions are not within the permission limit of the repository policy` | 許可 Caller の権限の上限が許さない権限名か水準を要求した | ワークフロー利用者に要求を直してもらうか、リポジトリの管理者に権限の上限を広げるかを判断してもらう |

### 4.2 Protected Repository Mismatch

`issuance_verification_failed` の `reason` が `protected repositories[<J>]` を含むときは、GitHub のリポジトリの名前と Repository ID の組が、保護対象の一覧と食い違っています。
GitHub 側でトークンが一度発行され、GATES は返さずに失効を試みています。
一覧を直すまで、その保護対象への要求は失敗し続けます。

| `reason` に含まれる文言 | 原因 | 直し方 |
|---|---|---|
| `protected repositories[<J>] matches the repository id but not the repository name` | 保護対象を改名したか、一覧の `name` を書き誤った | `name` を現在の名前に直す |
| `protected repositories[<J>] matches the repository name but not the repository id` | 保護対象を削除して同じ名前で作り直したか、一覧の `id` を書き誤った | 保護するなら、`id` を現在の Repository ID に直す。保護しないなら、一覧から外す |

要求したリポジトリは `request.repositories` にあります。
その完全名で、現在の Repository ID と名前を調べます。

```bash
gh api repos/<オーナー名>/<リポジトリ名> --jq '[.id, .name] | @tsv'
```

直したら、`make tf-plan` で差分を確かめてから適用します。

## 5. Failures without a request_id

要求が GATES に届かなかったか、GATES が応答を書けなかった失敗です。

### 5.1 AssumeRoleWithWebIdentity failed

Action が IAM Role を引き受けられませんでした。
§1 の原因のほかに、次を確かめます。

- AWS アカウントに IAM OIDC Provider があり、Client ID に `sts.amazonaws.com` がある
- `allowed_owner_id` と `allowed_owner_name` が、許可オーナーの値と一致する。許可オーナーが改名したなら、`allowed_owner_name` を新しい名前にして適用し直す

失敗した時刻の前後で、引き受けの記録を CloudTrail から引きます。
`--region` は、ワークフローの `aws-region` のリージョンにします。

```bash
aws cloudtrail lookup-events --region <リージョン> \
  --lookup-attributes AttributeKey=EventName,AttributeValue=AssumeRoleWithWebIdentity \
  --start-time <開始> --end-time <終了> \
  --query 'Events[].CloudTrailEvent' --output json \
  | jq -r '.[] | fromjson | [.eventTime, (.errorCode // "success"), .userIdentity.userName] | @tsv'
```

`userIdentity.userName` は、OIDC JWT の `sub` です。

- `repo:` の後に `@` と Owner ID がない：リポジトリの OIDC JWT が名前だけの形式。リポジトリか Organization の設定で切り替える
- オーナーが許可オーナーでない：許可オーナー以外のリポジトリからの呼び出し
- 記録がない：`role-arn` が別のアカウントを指しているか、`aws-region` が誤っている

### 5.2 the Lambda Invoke failed

AWS Lambda が要求を関数に渡しませんでした。
関数のログにも、既定の CloudTrail にも残らないため、注釈の AWS のエラーコードで見分けます。

| AWS のエラーコード | 原因 | 直し方 |
|---|---|---|
| `AccessDeniedException` | `function-name` か `aws-region` の誤り。存在しない関数もこのコードになる | ワークフロー利用者に、登録した Variables を渡しているかを確かめてもらう |
| `TooManyRequestsException` | 同時実行数の上限を超えた | `aws lambda get-function-concurrency --function-name gates` で予約を確かめる。`0` なら、悪用への対処で意図して止めたものでないかを確かめてから、`lambda_reserved_concurrency` を見直して適用する |

### 5.3 the Lambda function returned FunctionError

関数は動いたものの、応答を書けずに終わりました。
注釈の `errorType` で見分けます。

| `errorType` | 原因 | 直し方 |
|---|---|---|
| `Runtime.ExitError` | 関数の初期化の失敗。関数の設定の環境変数の欠落か不正な値（発行上限の `issuable_permissions` に GATES が発行できない組がある、`protected_repositories` に `make tf-plan` を通る誤りがある、など）で起きる | ログの `initialization failed` の行に、満たさなかった設定の名前と条件が出る。発行上限と保護対象の一覧の誤りの読み方は下記にある。`make tf-plan` で差分を確かめてから適用し直す |
| `Sandbox.Timedout` | 関数のタイムアウト | ログの `REPORT` の行に `Status: timeout` が出る。タイムアウトが Terraform の値（45 秒）から変わっていれば、適用し直す |

時刻でログを引きます。
`<開始>` と `<終了>` は、ミリ秒の UNIX 時刻です。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --query 'events[].message' --output text
```

発行上限の誤りでは、`initialization failed` の行の `error` が次の形になります。

```text
invalid config: GATES_ISSUABLE_PERMISSIONS: invalid issuable permissions: entries[1] is not in allowed permissions; entries[2] is not in allowed permissions
```

`entries[N]` は、`terraform.tfvars` の `issuable_permissions` のリストの、0 から数えた位置です。
ログには値が出ないため、位置でリストの要素を探します。
誤った組がいくつあっても、`; ` で区切って一行に並びます。
`is not in allowed permissions` は、その組が Action Reference の「要求できる権限」にないことを示します（`members:read` のような Repository permissions でない権限、`contents:admin` のような水準、綴りの誤り）。
この組を除くか直して、適用し直します。

保護対象の一覧の誤りでは、`error` が次の形になります。

```text
invalid config: GATES_PROTECTED_REPOSITORIES: invalid protected repositories: repositories[0].callers[0].permissions: invalid permission limit: entries[1] is not in allowed permissions
```

誤りの位置は、`repositories[0].callers[0].permissions` のような要素のパスか、`line 5:` のような行で示されます。
行は、`protected_repositories` のヒアドキュメントの中の、1 から数えた行です。
`permissions` の `entries[N]` は、書いた順に 0 から数えた組の位置です。
文字の誤り（UTF-8 として正しくない、BOM などの見えない文字がある）は、位置を示しません。
ログには Repository ID、名前、権限の値が出ません。
誤りがいくつあっても `; ` で区切って一行に並び、発行上限の誤りもあれば、環境変数ごとに改行で分かれます。

| 文言 | 誤り |
|---|---|
| `is not in allowed permissions` | 権限の上限に、Action Reference の「要求できる権限」にない組がある |
| `is a duplicate key` | 同じまとまり（最上位、一つの保護対象、一つの許可 Caller、一つの `permissions`）の中に同じキーが 2 つある |
| `must not have an anchor`、`must not be an alias`、`must not have a merge key`、`must not have an explicit tag`、`must not have a directive` | アンカー、エイリアス、マージキー、タグ、`%TAG` などの行がある |
| `must be a string` | 引用符のない名前が、文字列以外に読める（`123`、`true` など）。名前を引用符で囲む |
| `has a value that is not a string` | 権限の上限の水準が、文字列以外に読める（`contents: true` など）。「要求できる権限」の水準に直す |
| `must be valid UTF-8`、`must not contain a byte order mark`、`must not contain NEL, LS, or PS` | UTF-8 として正しくない文字か、見えない文字が入っている。保護対象の一覧を UTF-8 で打ち直す |
| `GATES_PROTECTED_REPOSITORIES: is not set` | 関数の設定に一覧がない。Terraform の外で関数の設定が変えられた可能性がある |

### 5.4 Other Annotations

`the Lambda response` か `the success response has no usable` で始まる注釈は、Action が GATES の応答を読めなかったことを示します。
Action の参照先が、デプロイした GATES と同じリリースのコミットかを確かめます。

次の注釈は、Runner の環境か Action の想定外の失敗を示し、AWS に手がかりがありません。

- **Runner の環境**：`ACTIONS_ID_TOKEN_REQUEST_URL is not set`、`GITHUB_OUTPUT is not set`、`GITHUB_STATE is not set`、`invalid ACTIONS_ID_TOKEN_REQUEST_URL`、`could not write to GITHUB_STATE`、`could not write to GITHUB_OUTPUT`
- **Action の想定外の失敗**：`the Lambda Invoke returned StatusCode`、`the child process could not be started`、`the child process terminated unexpectedly`、`an unexpected failure occurred`

注釈の全文とワークフロー実行の URL を添えて、Runner の管理者か GATES の開発者に伝えます。

## 6. Revocation Warning

`revocation failed:` で始まる `::warning::` は、ジョブの終了時にトークンを失効できなかったことを示します。
ジョブは失敗せず、トークンは期限（発行から 1 時間）を過ぎれば使えなくなります。
残ったトークンで操作できるのは、要求したリポジトリに対する、要求した権限の範囲です。
書き込みの権限を要求していれば、期限まで書き込めます。

セルフホストの Runner では、`api.github.com` への通信の許可を確かめます。
GitHub がホストする Runner で繰り返し出るときは、注釈の全文とワークフロー実行の URL を運用者に伝えます。

同じステップに `::error::` の注釈もあれば、そちらから §1 以降を引きます。

## 7. References

- [Action Reference](action-reference.md)：Action の入力の書式と検証、要求できる権限
