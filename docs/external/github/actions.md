# GitHub Actions Integration Constraints

この文書は、GitHub.com の GitHub Actions で Action とワークフローを動かすときの仕様を、GitHub の公式資料と、GitHub Actions の実行の調査記録を根拠に整理したものです。
GitHub の公式資料そのものではなく、GitHub の仕様が変更された場合は該当する公式資料を確認します。

- **対象**：GitHub.com の GitHub Actions における OIDC JWT の取得、Composite Action の Output とマスク、Secrets と Variables、Dependabot、Docker Action、Runner の上のジョブ。GitHub Enterprise Server は対象外。
- **取得した値の調査記録**：[GitHub Composite Action Outputs](../../research/github/composite-action-outputs.md)、[GitHub Docker Action Runtime](../../research/github/docker-action-runtime.md)、[GitHub Docker Action Failure and Cancellation](../../research/github/docker-action-cancellation.md)。取得日、取得方法、記録した応答はこれらの記録にある。

各記述の先頭の太字は、根拠の区分（公式仕様、公開値、実測値、推論、未確認）を表します。

## 1. OIDC JWT の取得

**公式仕様**：GitHub Actions が OIDC JWT を取得するには、ワークフローまたはジョブで `permissions: id-token: write` を付与します。ランナーでは `ACTIONS_ID_TOKEN_REQUEST_URL` と `ACTIONS_ID_TOKEN_REQUEST_TOKEN` による取得方法も提供され、要求時に Audience を指定できます（[OIDC reference](https://docs.github.com/en/actions/reference/security/oidc#workflow-permissions-for-the-requesting-the-oidc-token)、[OIDC reference — Methods for requesting the OIDC token](https://docs.github.com/en/actions/reference/security/oidc#methods-for-requesting-the-oidc-token)）。

## 2. Output とマスク

**公式仕様**：Composite Action では、`GITHUB_OUTPUT` へ書き込んだ値を、`outputs.<output_id>.value` で Action の Output として後続ステップへ公開できます。ログに表示させたくない値は `add-mask` でマスク対象に登録できます（[Workflow commands](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)、[Metadata syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/metadata-syntax)）。

**公式仕様（マスクと Output）**：マスクは、ジョブごと、値ごとに登録が必要です。
同じ資料は、マスクした値は Output に設定できないと説明する一方で、同一ジョブ内なら生成、`add-mask`、`GITHUB_OUTPUT` の順で後続ステップへ渡す例を示しています。
ジョブやワークフローをまたいでマスクした値を渡す場合は、シークレットストアを経由するよう案内しています（[Workflow commands](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)）。

**実測値（マスクと Output）**：リポジトリ内の Composite Action の内部ステップで `add-mask` した値を `GITHUB_OUTPUT` に書き、`action.yml` の `outputs` で公開した記録では、呼び出し元の同じジョブの後続ステップが値を欠けずに受け取り、その値はログで `***` にマスクされました。
呼び出し元は `add-mask` を実行していません。
`name=value` の形式と区切り文字の形式の両方で、`+`、`/`、`=` を含む64文字の値と、500文字の値が、どちらも同じ値で届きました。
同じ値をジョブの `outputs` に置くと、GitHub は「Skip output ... since it may contain secret」の警告を出して設定せず、次のジョブでは空文字列でした（[GitHub Composite Action Outputs](../../research/github/composite-action-outputs.md)）。

**推論**：公式資料の「マスクした値は Output に設定できない」は、この観測と照らすと、ジョブの Output を指すと読めます。

## 3. Secrets と Variables

**公式仕様**：GitHub Actions の Secrets / Variables はリポジトリ、Environment、Organization の単位で設定できます。Organization の Secrets / Variables はポリシーで利用できるリポジトリを制限でき、GitHub Free の private リポジトリからは Organization の Secrets / Variables を利用できません（[Using secrets](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-secrets)、[Variables reference](https://docs.github.com/en/actions/reference/workflows-and-actions/variables)）。

**推論**：個人アカウントが所有するリポジトリでは Organization 単位の設定を使えないため、値はリポジトリまたは Environment の単位で設定します。

## 4. Dependabot

**公式仕様（Dependabot が起動したワークフロー）**：Dependabot が `push`、`pull_request`、`pull_request_review`、`pull_request_review_comment` で起動したワークフローは、既定ではフォークから開いたものと同じに扱われ、`GITHUB_TOKEN` は読み取り専用になります。`permissions` キーで権限を引き上げられます。
参照できる Secrets は Dependabot の Secrets だけで、GitHub Actions の Secrets は参照できません。
Dependabot の Secrets は `secrets` コンテキストに同じ書き方で入ります（[Troubleshooting Dependabot on GitHub Actions](https://docs.github.com/en/code-security/reference/supply-chain-security/troubleshoot-dependabot/dependabot-on-actions)）。

**推論**：Dependabot が起動したワークフローでも、Variables は参照できます。上記の資料が参照を制限するのは Secrets と `GITHUB_TOKEN` の権限で、Variables は制限の対象に挙げていないためです。

**公式仕様（Dependabot の `github-actions` の対象）**：`package-ecosystem: github-actions` の `directory` に `/` を指定すると、Dependabot は `/.github/workflows` と、ルートの `action.yml` / `action.yaml` を探します。
`directories` はグロブとワイルドカード `*` を受け付け、`directory` は受け付けません（[Dependabot options reference](https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference)）。

**公式仕様（Dependabot の `docker`）**：`package-ecosystem: docker` は `Dockerfile` のイメージのタグを更新します（[Dependabot options reference](https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference)、[Dependabot supported ecosystems and repositories](https://docs.github.com/en/code-security/reference/supply-chain-security/supported-ecosystems-and-repositories)）。
docs.github.com の資料は、`FROM` の digest を更新するとは明記していません。

## 5. Docker Action

**公式仕様（Docker Action）**：Docker Action は Linux の Runner だけで動き、セルフホストの Runner には Linux と Docker が要ります（[About custom actions](https://docs.github.com/en/actions/concepts/workflows-and-actions/custom-actions)）。
`runs.image` に `Dockerfile` を指定すると、Runner がそのファイルからイメージをビルドします。パスは `action.yml` からの相対パスで、ファイル名は `Dockerfile` でなければなりません。
入力は `INPUT_<大文字の入力名>` の環境変数で渡され、空白だけが `_` に変わります。
`runs.args` はコンテナの `ENTRYPOINT` に渡す引数で、`Dockerfile` の `CMD` を置き換え、`runs.entrypoint` は `ENTRYPOINT` を置き換えます。
`runs.post-entrypoint` は、`runs.entrypoint` の後に同じイメージの新しいコンテナで動き、状態はワークスペース、`HOME`、`STATE_` の環境変数で受け渡します。`runs.post-if` の既定は `always()` です（[Metadata syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/metadata-syntax)）。
`GITHUB_STATE` に書いた値は、同じ Action の `pre:` または `post:` で `STATE_<名前>` の環境変数として読めます（[Workflow commands](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)）。
Docker Action は既定の Docker ユーザー（root）で動かす必要があり、`USER` を使うと `GITHUB_WORKSPACE` にアクセスできなくなります。
`WORKDIR` は使わず、`ENTRYPOINT` は exec 形式と絶対パスで書くよう推奨しています。exec 形式では `args` がシェルを通らず、環境変数が展開されません（[Dockerfile support for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/dockerfile-support)）。

**実測値（Docker Action）**：リポジトリ内の Docker Action を `args` なしで動かした記録では、`role-arn` の入力は `INPUT_ROLE-ARN` の環境変数で届き、ハイフンは残りました。
`id-token: write` のあるジョブでは `ACTIONS_ID_TOKEN_REQUEST_URL` と `ACTIONS_ID_TOKEN_REQUEST_TOKEN` がコンテナに渡り、コンテナの中から `sts.amazonaws.com` を Audience とする OIDC JWT を取得できました。ないジョブでは両方とも渡りませんでした。
`ACTIONS_RUNTIME_TOKEN` はどちらのジョブでも渡りました。
コンテナには `/var/run/docker.sock`、`RUNNER_TEMP`、`HOME`、ファイルコマンドのディレクトリ、ワークスペースがマウントされ、`docker run` に `--user` は付きませんでした。
`USER 65534:65534` のイメージでは、`GITHUB_OUTPUT`、`GITHUB_STATE`、`GITHUB_ENV` に書き込めず（所有者は Runner のユーザーで、ほかのユーザーに書き込み権限がない）、標準出力のワークフローコマンドだけが効きました。
root では、メインで `add-mask` した値が `GITHUB_OUTPUT` を経て呼び出し元の後続ステップに欠けずに届いてマスクされ、`GITHUB_STATE` を経て post-entrypoint に `STATE_` の環境変数として届いてマスクされました。
メインが終了コード 1 で終えたステップ（`continue-on-error: true`）でも、post-entrypoint は動きました（[GitHub Docker Action Runtime](../../research/github/docker-action-runtime.md)）。

**実測値（失敗とキャンセル）**：`continue-on-error` を付けないメインの失敗でジョブが失敗した場合、後続のステップの失敗でジョブが失敗した場合、後続のステップの実行中にワークフロー実行をキャンセルした場合のどれでも、post-entrypoint は動きました。
ワークフロー実行のキャンセルの API を呼んでから Runner がステップを終えるまでに、12〜13 秒かかりました。
`docker run` のプロセスに送った SIGINT は、コンテナの中のプロセスに届きました（[GitHub Docker Action Failure and Cancellation](../../research/github/docker-action-cancellation.md)）。

**推論（Docker Action の実行ユーザー）**：公式資料が `USER` を禁じる理由に挙げるのは `GITHUB_WORKSPACE` だけですが、観測ではファイルコマンドにも書き込めませんでした。ファイルコマンドの所有者は Runner を動かすユーザーで、その uid は Runner の環境ごとに決まるため、Action のイメージが固定の uid で書き込める保証はありません。Output と `STATE_` を使う Docker Action は、root で動かす前提になります。

## 6. Runner の上のジョブ

**公式仕様（Runner の上のジョブ）**：GitHub がホストする Runner は、使い捨ての隔離された仮想マシンでコードを実行します。
ワークフローの中のジョブは、後のジョブが使う環境変数を読む、後のジョブが処理する共有のディレクトリに書く、Docker ソケットを通じてほかのコンテナを検査してその中でコマンドを実行する、といった形で互いに影響し得ます（[Secure use reference](https://docs.github.com/en/actions/reference/security/secure-use)）。
Linux と macOS の仮想マシンは、パスワードなしの `sudo` で動きます（[GitHub-hosted runners reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)）。

**実測値（Runner の上の境界）**：`ubuntu-latest`（Ubuntu 26.04）と Runner 2.338.2 の記録では、root で動く Docker Action のコンテナは、マウントされた Docker ソケットを通じてホストの `/` をマウントしたコンテナを起動し、ホストのファイルを読めました。
ソケットの所有グループは `docker` で、そのグループに属さない uid 65534 のユーザーは接続できず、Runner のユーザーはそのグループに属していました。
Runner はステップごとのファイルコマンドのファイルをステップの後も消さず、後続のステップは前のステップの Output と `GITHUB_STATE` の値を読めました。
前のステップが起動したバックグラウンドのプロセスは、ジョブの終わりまで動き続けました。
`id-token: write` のジョブでは、`run` のステップに `ACTIONS_ID_TOKEN_REQUEST_URL` と `ACTIONS_ID_TOKEN_REQUEST_TOKEN` が渡り、`ACTIONS_RUNTIME_TOKEN` は渡りませんでした。
コンテナの中の `ACTIONS_RUNTIME_TOKEN` と `ACTIONS_ID_TOKEN_REQUEST_TOKEN` は同じ値で、`run` のステップの要求用トークンも同じ値でした（[GitHub Docker Action Runtime](../../research/github/docker-action-runtime.md#runner-boundary)）。

## 7. Outstanding Verification

公式資料で確定できていない事項を次にまとめます。根拠と確認範囲は各セクションに記載しています。
公開値や実測値で一部の条件を確かめた事項も、GitHub の保証がないためここに残します。

| 項目 | 確定できていない内容 |
|---|---|
| Docker Action | ジョブが失敗した状態とキャンセルした状態での post-entrypoint の実行、複数の Action の post が動く順序、別のリポジトリを `uses:` で参照したときのビルドの時間と post-entrypoint でのビルドのし直し、Dependabot の `docker` が `FROM` のタグと並べた digest を更新するか、Runner のバージョンやイメージが変わったときのソケットの所有グループ、ファイルコマンドのファイルをステップの後に残すこと、`ACTIONS_RUNTIME_TOKEN` と要求用トークンが同じ値であること |
