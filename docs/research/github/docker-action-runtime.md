# GitHub Docker Action Runtime

本ドキュメントでは、リポジトリ内の Docker Action（`runs.using: docker`、`image: Dockerfile`）を GitHub Actions の実環境で動かしたときの挙動を記述します。
調べたのは、入力の受け取り方、OIDC JWT の取得、コンテナのマウント、実行ユーザー、Output とマスク、post-entrypoint への状態の受け渡し、イメージのビルドの時間です。
あわせて、コンテナからマウントされた Docker ソケットで何ができるか、同じジョブのほかのステップから何が見えるか、GATES の Docker Action が Runner に何を残すかを調べました（[Runner Boundary](#runner-boundary)）。
観測と合わせて、actions/runner v2.338.2 のソースから読み取れる実装も記録します。

- 調査日：**2026/11/09**（Runner Boundary は **2026/11/14**）
- 検証環境：個人アカウントのプライベートリポジトリ。`runs-on: ubuntu-latest`
- 起動：作業ブランチへの push

オーナー名は `octocat`、リポジトリ名は `example-repo`、ブランチ名は `example-branch` に置き換えています。
Output、`GITHUB_STATE`、`GITHUB_ENV` に書いた値は、コンテナの中で `/dev/urandom` から作った乱数の文字列と、その SHA-256 のハッシュ値だけです。
OIDC JWT、その要求に使うトークン（`ACTIONS_ID_TOKEN_REQUEST_TOKEN`）、`ACTIONS_RUNTIME_TOKEN`、GATES が発行したトークンの値は、ログにも本ドキュメントにも出していません。
ログに `eyJ` で始まる文字列、`ghs_`、`Bearer` の後に続く値がないことを、実行後に確かめました。

runner-images の README の表からリンクされたイメージごとのソフトウェア一覧では、Ubuntu 24.04 と 26.04 のどちらのイメージにも Docker の Server と Client があります。
`ubuntu-latest` は、このどちらかを指します。

## Probe

調査用の Action を置き、ワークフローの各ジョブから `uses: ./.github/actions/<名前>` で呼びました。
`uses:` で指定したディレクトリの `action.yml` と `Dockerfile` が使われ、ビルドの引数を渡す手段がないため、変種ごとにディレクトリを分けています。

| Action | イメージ | 実行ユーザー | 調べたこと |
|---|---|---|---|
| `probe-docker` | Alpine 3.24.2 と、`entrypoint.sh`、`post.sh` | 既定（root） | 入力、OIDC、マウント、Output、マスク、post-entrypoint |
| `probe-docker-nonroot` | 同上に `USER 65534:65534` を加えたもの | 65534 | root 以外での書き込み |
| `probe-docker-go` | `golang:1.27.3-alpine3.24` でビルドし、`scratch` に実行ファイルと CA 証明書を置くもの | 既定（root） | ビルドの時間 |
| `probe-docker-go-debian` | 同上で、ビルド用のイメージを `golang:1.27.3-trixie` にしたもの | 既定（root） | ビルドの時間 |

| ジョブ | Action | `permissions` | そのほかの条件 |
|---|---|---|---|
| `with-id-token` | `probe-docker` | `contents: read`、`id-token: write` | |
| `without-id-token` | `probe-docker` | `contents: read` | |
| `main-fails` | `probe-docker` | `contents: read` | 入力 `fail: "true"` でメインを終了コード 1 で終える。ステップに `continue-on-error: true` |
| `nonroot` | `probe-docker-nonroot` | `contents: read`、`id-token: write` | ステップに `continue-on-error: true` |
| `go-build-alpine` | `probe-docker-go` | `contents: read` | |
| `go-build-debian` | `probe-docker-go-debian` | `contents: read` | |

`action.yml` は `args` を書かず、`entrypoint` と `post-entrypoint` に `Dockerfile` が置いたスクリプトの絶対パスを書きました。

## Inputs

入力は、`args` なしで環境変数としてコンテナに届きました。
`role-arn` という入力は `INPUT_ROLE-ARN` の名前で届き、ハイフンは `_` に変わりませんでした。
値のハッシュは、呼び出し元で渡した文字列のハッシュと一致しました。

```text
argument count: 0
INPUT_FAIL
INPUT_ROLE-ARN
INPUT_ROLE_ARN length: 0
```

ハイフンを含む名前はシェルの変数名として参照できないため、スクリプトでは `printenv 'INPUT_ROLE-ARN'` で読みました。
Go のプログラムでは `os.Environ()` に `INPUT_ROLE-ARN` が現れました。

Runner はコンテナを次の形で起動しました（抜粋）。
入力とトークンの環境変数は `-e "<名前>"` で名前だけを渡し、値は `docker run` の行に現れませんでした。
`--user` は付いていません。

```text
/usr/bin/docker run --name <名前> --label <ラベル> --workdir /github/workspace --rm
  -e "INPUT_ROLE-ARN" -e "INPUT_FAIL" -e "HOME" ... -e "GITHUB_OUTPUT" ... -e "RUNNER_TEMP" ...
  -e "ACTIONS_RUNTIME_URL" -e "ACTIONS_RUNTIME_TOKEN" -e "ACTIONS_CACHE_URL"
  -e "ACTIONS_ID_TOKEN_REQUEST_URL" -e "ACTIONS_ID_TOKEN_REQUEST_TOKEN"
  -e "ACTIONS_RESULTS_URL" -e "ACTIONS_CACHE_MODE" -e "ACTIONS_ORCHESTRATION_ID"
  -e GITHUB_ACTIONS=true -e CI=true --entrypoint "/entrypoint.sh"
  -v "/var/run/docker.sock":"/var/run/docker.sock"
  -v "/home/runner/work/_temp":"/github/runner_temp"
  -v "/home/runner/work/_temp/_github_home":"/github/home"
  -v "/home/runner/work/_temp/_github_workflow":"/github/workflow"
  -v "/home/runner/work/_temp/_runner_file_commands":"/github/file_commands"
  -v "/home/runner/work/example-repo/example-repo":"/github/workspace"
  <イメージ>
```

## OIDC

`id-token: write` の有無で、OIDC JWT の要求に使う環境変数がコンテナに渡るかが分かれました。
`ACTIONS_RUNTIME_TOKEN` は、`id-token: write` の有無によらず渡りました。

| 環境変数 | `with-id-token` | `without-id-token` |
|---|---|---|
| `ACTIONS_ID_TOKEN_REQUEST_URL` | あり | なし |
| `ACTIONS_ID_TOKEN_REQUEST_TOKEN` | あり | なし |
| `ACTIONS_RUNTIME_TOKEN` | あり | あり |
| `ACTIONS_RUNTIME_URL`、`ACTIONS_RESULTS_URL`、`ACTIONS_CACHE_URL` | あり | あり |
| `GITHUB_TOKEN` | なし | なし |

`with-id-token` のコンテナからは、`sts.amazonaws.com` を Audience とする OIDC JWT を `200` で取得できました。
OIDC JWT の要求に使うトークンは `Authorization: Bearer` のヘッダーで送りました。
uid 65534 で動かした `nonroot` のコンテナでも、同じ要求が `200` で JWT を返しました。

## Mounts

コンテナには、上の `docker run` の `-v` のとおりにマウントされました。

- **Docker ソケット**：`/var/run/docker.sock` はソケットとして存在し、所有者は uid 0、グループは gid 113、権限は `srw-rw----` だった。権限から見て、root で動くコンテナはこのソケットに書き込める。ソケットを通じた操作は [Runner Boundary](#docker-socket) に記録した
- **`RUNNER_TEMP` と `HOME`**：コンテナの中ではそれぞれ `/github/runner_temp` と `/github/home` を指す。どちらもホストの `/home/runner/work/_temp` とその下のディレクトリをそのままマウントしたもので、ほかのステップと共有される。メインが `HOME` に置いたファイルは、post-entrypoint のコンテナからも見えた
- **ファイルコマンド**：`GITHUB_OUTPUT`、`GITHUB_STATE`、`GITHUB_ENV` は `/github/file_commands/` の下のファイルを指す。ファイルの所有者は uid 1001、gid 1001 で権限は `-rw-r--r--`、ディレクトリは `drwxr-xr-x` だった。ワークスペースと `RUNNER_TEMP` も同じ uid 1001 が所有する

## Non-root User

`USER 65534:65534` を置いたイメージでは、プロセスは uid 65534、gid 65534 で動き、ファイルコマンドに書き込めませんでした。
ファイルの所有者が uid 1001 で、ほかのユーザーに書き込みの権限がないためです。

```text
uid: 65534, gid: 65534
/entrypoint.sh: line 76: can't create /github/file_commands/set_output_<UUID>: Permission denied
/entrypoint.sh: line 81: can't create /github/file_commands/save_state_<UUID>: Permission denied
/entrypoint.sh: line 86: can't create /github/file_commands/set_env_<UUID>: Permission denied
GITHUB_OUTPUT: not writable
GITHUB_STATE: not writable
GITHUB_ENV: not writable
HOME: not writable
GITHUB_WORKSPACE: not writable
```

標準出力のワークフローコマンドは、uid 65534 でも効きました。
`::notice::` は注釈になり、`::add-mask::` した値は `***` で表示されました。

```text
##[notice]notice emitted by uid 65534
echo inside main (value): ***
```

スクリプトは書き込みの失敗で終了しなかったため、メインのステップは `success` で終わりました。
呼び出し元の後続ステップでは Output が空文字列（長さ 0）で、`GITHUB_ENV` に書こうとした変数も未設定でした。
post-entrypoint も uid 65534 で動き、`STATE_` の環境変数は 1 つもなく、メインが `HOME` に置こうとしたファイルもありませんでした。

GitHub の Dockerfile support for GitHub Actions は、Docker Action を既定の Docker ユーザー（root）で動かすよう求め、`USER` を使うと `GITHUB_WORKSPACE` にアクセスできなくなると説明しています。
観測では、`GITHUB_WORKSPACE` に加えて、`GITHUB_OUTPUT`、`GITHUB_STATE`、`GITHUB_ENV`、`HOME` にも書き込めませんでした。

## Output and Mask

メインのコンテナで 64 文字の乱数の値を `::add-mask::` で登録し、`GITHUB_OUTPUT` に `probe-value=<値>` の 1 行で書きました。
値の SHA-256 は、マスクせずに別の Output に書きました。
呼び出し元の後続ステップは `steps.probe.outputs.probe-value` を `env:` で受け取り、`echo` とハッシュの比較をしました。
呼び出し元は `::add-mask::` を実行していません。

```text
echo inside main (value): ***
```

```text
env:
  PROBE_VALUE: ***
  PROBE_SHA256: <64 文字の 16 進数>
probe-value: ***
probe-value: match (length 64)
```

`with-id-token` と `without-id-token` のどちらでも、値は欠けずに届き、ログでは `***` にマスクされました。
`with-id-token` では、`GITHUB_ENV` に書いた変数も、呼び出し元の後続ステップの環境変数になりました。
`without-id-token` では、この点を確かめていません。

失敗の前に書いた Output も、呼び出し元に届きました。
`main-fails` では、メインが Output を書いた後に終了コード 1 で終えました。
ステップの `outcome` は `failure` で、後続ステップが受け取った `probe-sha256` の Output は 64 文字でした。

## Post-entrypoint

post-entrypoint は、ジョブの最後に `Post <ステップ名>` のステップとして動きました。
同じ `Dockerfile` からイメージをビルドし直し、その新しいコンテナで動きます。
同じ Runner の中でのビルドなので、どの層も `CACHED` で、ビルドは 1 秒未満で終わりました。
`docker run` の行には、メインが `GITHUB_STATE` に書いた `STATE_probe_state` と `STATE_probe_state_sha256`、`GITHUB_ENV` に書いた `PROBE_ENV_SHA256` が加わりました。
`--entrypoint` は `"/post.sh"` でした。
`INPUT_` の環境変数と、OIDC JWT の要求に使う環境変数は、メインと同じく渡りました。

メインで `::add-mask::` した値を `GITHUB_STATE` に `probe_state=<値>` で書き、post-entrypoint で `STATE_probe_state` から読みました。

```text
== post: names of STATE_* variables
STATE_probe_state
STATE_probe_state_sha256
echo inside post (state): ***
state length: 64, sha256: <ハッシュ>, expected: <同じハッシュ>
state: match
```

値は欠けずに届き、メインで登録したマスクが post-entrypoint のログにも効きました。
`STATE_` の後の名前は、`GITHUB_STATE` に書いた名前の大文字と小文字をそのまま保ちました。

`main-fails` でも post-entrypoint は動き、`STATE_` の値はハッシュが一致しました。
このジョブはメインのステップに `continue-on-error: true` を付けているため、ジョブそのものは失敗していません。
ジョブが失敗した状態での post-entrypoint の実行は確かめていません。

## Build Time

同じリポジトリの Action を `uses: ./<パス>` で参照したときは、ステップの中で `docker build` が動き、その時間はステップの時間に含まれます。

`probe-docker-go` と `probe-docker-go-debian` がビルドする Go のプログラムは、AWS SDK の `service/sts` と `service/lambda` を import します。
どちらも 1 回だけの計測で、ばらつきは測っていません。

| Action | ビルド用のイメージ | ステップの時間 | `go build` | ビルド用のイメージの取得と展開 |
|---|---|---|---|---|
| `probe-docker-go` | golang alpine | 43 秒 | 30 秒 | 約 8 秒 |
| `probe-docker-go-debian` | golang trixie | 44 秒 | 22 秒 | 約 18 秒 |

Alpine にシェルスクリプトを置くだけの `probe-docker` では、ステップの時間はジョブによって 3〜5 秒で、そのうちビルドは約 3 秒でした。

ビルド中の Docker Hub からのイメージの取得と `go mod download` は、どちらも成功しました。
別のリポジトリを `uses:` で参照したときのビルドの時間は計測していません。

## Runner Boundary

Docker Action のコンテナと、同じジョブのほかのステップのあいだに何があるかを調べました。
調査用の Docker Action（`probe-runner-boundary`）を、ワークフロー `Runner Boundary` の 1 つのジョブ（`id-token: write`）から呼びました。
このワークフローを 2 回動かし、2 回目は要求用トークンの比較を加えました。
GATES の Docker Action が Runner に残すものは、別のワークフロー `Runner Boundary GATES` で調べました（[Files Left by the GATES Action](#files-left-by-the-gates-action)）。

Runner は 2.338.2 で、`ubuntu-latest` は Ubuntu 26.04（イメージのバージョン 20261111.196.1）、Docker は 29.4.2（Engine API 1.54）でした。
GitHub の Secure use reference は、GitHub がホストする Runner は使い捨ての隔離された仮想マシンでコードを実行すると説明しています。
下のソケットの操作は、その仮想マシンの中にとどまります。

### Docker Socket

ソケットの所有者は uid 0、グループは gid 113、権限は `660` でした。
gid 113 はホストの `docker` グループで、Runner のユーザー `runner`（uid 1001）はその一員でした。
`run` のステップでは、パスワードなしの `sudo` も通りました。

```text
runner user: uid=1001(runner) gid=1001(runner) groups=1001(runner),4(adm),100(users),113(docker),999(systemd-journal)
srw-rw---- 1 0 113 0 Nov 14 07:55 /var/run/docker.sock
docker:x:113:runner
sudo -n: allowed
```

GitHub の GitHub-hosted runners reference は、Linux の仮想マシンがパスワードなしの `sudo` で動くと説明しています。
同じジョブの `run` のステップは、Docker Action を経由しなくても、ソケットとホストの root の権限を持ちます。

root で動く調査用のコンテナ（uid 0。補助グループに 113 はない）から、ソケットに Docker Engine API の要求を送りました。
コンテナの検査（`GET /containers/{id}/json`）は、コンテナの環境変数にランタイムのトークンが入るため送っていません。

| 要求 | 結果 |
|---|---|
| `GET /version` | `200`。Engine 29.4.2、API 1.54 |
| `GET /containers/json` | `200`。動いているコンテナは 1 つで、調査用の Action 自身のコンテナだった |
| `POST /containers/create`、`start`、`wait`、`logs` | 作成は `201`、起動は `204`。ホストの `/` を読み取り専用で `/host` にマウントし、ネットワークを外したコンテナを、調査用の Action 自身のイメージで起動した。終了コードは 0 |
| `DELETE /containers/{id}` | `204`。起動したコンテナを削除した |

起動したコンテナは root で動き、ホストの `/etc/os-release` と Runner の作業ディレクトリを読めました。
調査用の Action のイメージは Alpine のため、Ubuntu の値はホストのファイルのものです。

```text
PRETTY_NAME="Ubuntu 26.04.1 LTS"
VERSION_ID="26.04"
ID=ubuntu
host /home/runner/work: present
uid in the new container: 0
```

同じコンテナの中から、`su-exec` で root 以外のユーザーに切り替えて `GET /version` を送りました。
接続できるかは、ソケットの所有グループ（113）に属するかで分かれました。

| ユーザー | 結果 |
|---|---|
| uid 65534、gid 65534、補助グループは 65534 だけ | 接続できない（`curl: (7) Failed to connect to localhost:80 over unix:///var/run/docker.sock after 0 ms: Could not connect to server`） |
| uid 65534、gid 113、補助グループは 113 だけ | `200` |

### File Commands Across Steps

調査用の Action は、`::add-mask::` した 64 文字の乱数の値を `GITHUB_OUTPUT` に `probe_value=<値>` で、別の値を `GITHUB_STATE` に `probe_state=<値>` で書きました。
後続の `run` のステップは、`RUNNER_TEMP` の下のファイルコマンドのディレクトリから、Action のステップのファイルを名前で探して読み、値の SHA-256 を照合しました。
その前のステップでは、ファイルコマンドのディレクトリのファイルを 0.1 秒ごとに写すプロセスを `nohup` でバックグラウンドに起動しておきました。

```text
GITHUB_OUTPUT set_output_<UUID>: readable, match (length 64)
GITHUB_STATE save_state_<UUID>: readable, match (length 64)
watcher copy of GITHUB_OUTPUT: readable, match (length 64)
watcher copy of GITHUB_STATE: readable, match (length 64)
watcher: still running
```

- **ステップの後**：Runner はステップごとに `set_output_<UUID>`、`save_state_<UUID>`、`set_env_<UUID>`、`add_path_<UUID>`、`step_summary_<UUID>`、`artifacts_<UUID>`、`artifacts_list_<UUID>` を作り、ステップが終わっても消さなかった。後続のステップの時点で、それまでのすべてのステップのファイルが残っていた。どのファイルも所有者は uid 1001 で、権限は `-rw-r--r--` だった
- **ステップの最中**：前のステップが起動したバックグラウンドのプロセスは、ステップが終わっても動き続け、Action のステップのあいだにそのファイルを写し取れた。Runner はこのプロセスを、ジョブの最後の `Complete job` で `Terminate orphan process` として終了させた

### Request Token in Run Steps

`id-token: write` のジョブの `run` のステップには、OIDC JWT の要求用の変数が渡り、`ACTIONS_RUNTIME_TOKEN` は渡りませんでした。
`run` のステップから要求用トークンで OIDC JWT を要求すると `200` が返りました。

```text
ACTIONS_ID_TOKEN_REQUEST_URL: present
ACTIONS_ID_TOKEN_REQUEST_TOKEN: present
ACTIONS_RUNTIME_TOKEN: absent
ACTIONS_RUNTIME_URL: absent
ACTIONS_RESULTS_URL: absent
oidc jwt request from a run step: HTTP status 200
request token equals the one in the probe container: yes
```

コンテナの中では、`ACTIONS_RUNTIME_TOKEN` と `ACTIONS_ID_TOKEN_REQUEST_TOKEN` が同じ値でした。
`run` のステップの要求用トークンも、コンテナの中の要求用トークンと同じ値でした。
どちらの比較も値を出さずに行いました。
コンテナの中では文字列を直接比べ、`run` のステップでは、コンテナが `RUNNER_TEMP` に置いた SHA-256 と自分の値の SHA-256 を比べ、一致したかだけを出しました。

```text
ACTIONS_RUNTIME_TOKEN equals ACTIONS_ID_TOKEN_REQUEST_TOKEN: yes
```

### Runner Process Environment

`run` のステップから、Runner のプロセス（`Runner.Listener` と `Runner.Worker`）の `/proc/<pid>/environ` を読み、環境変数の名前だけを出しました。
どちらのプロセスも所有者は `runner` で、`sudo` なしで読めました。
変数は 61 個と 63 個で、名前に `ACTIONS_ID_TOKEN_REQUEST_URL`、`ACTIONS_ID_TOKEN_REQUEST_TOKEN`、`ACTIONS_RUNTIME_TOKEN`、`GITHUB_TOKEN` はありませんでした。
`ACTIONS_` で始まる名前は `ACTIONS_RUNNER_ACTION_ARCHIVE_CACHE` と `ACTIONS_RUNNER_RETURN_JOB_RESULT_FOR_HOSTED` だけでした。
プロセスのメモリは調べていません。

### Files Left by the GATES Action

GATES の Docker Action を受け入れの対象の commit で参照し、対象 1 件の `contents:read` を要求しました。
Action の前後の `run` のステップで、`RUNNER_TEMP` と、`HOME`（`/home/runner`）のうち作業ディレクトリ（`/home/runner/work`）を除く部分のファイルの一覧を取って比べました。
Docker Action の `HOME`（`/github/home`）は `RUNNER_TEMP` の下の `_github_home` で、一覧に含まれます。
続いて、`RUNNER_TEMP` のすべてのファイルと、前の一覧の後に変わった `HOME` のファイルから、発行されたトークンの文字列を含むものを探し、ファイルの名前だけを出しました。
同じワークフローを 2 回動かし、同じ結果でした。

| 確認 | 結果 |
|---|---|
| `RUNNER_TEMP` に増えたファイル | Action のステップと後の一覧のステップのファイルコマンドのファイル、後の一覧のステップのスクリプト、空の `_github_home` のディレクトリ。Action のステップの `set_output_<UUID>` は 524 バイト、`save_state_<UUID>` は 397 バイトで、ほかのファイルコマンドのファイルは 0 バイト（`artifacts_list_<UUID>` は 27 バイト） |
| `HOME` で増えたか変わったファイル | 後の一覧のステップの `gh` が作った `~/.local/state/gh/device-id` と、Runner の診断ログ（`actions-runner/cached/2.338.2/_diag/` の下） |
| トークンを含む `RUNNER_TEMP` のファイル | Action のステップの `set_output_<UUID>` と `save_state_<UUID>` だけ |
| トークンを含む `HOME` のファイル | なし。Runner の診断ログも含まない |

トークンは後続のステップで使え（`README.md` の取得が `200`）、post-entrypoint は `revocation succeeded: HTTP status 204` で失効させました。
post-entrypoint はジョブの最後に動くため、その後のファイルは調べていません。
ジョブのログの検索（`ghs_`、`eyJ`、`ASIA`、`FwoG`、`IQoJ`）は 0 件で、同じ時間帯の GATES の関数のロググループにもこれらの文字列はなく、監査記録は 2 件の `success` でした。

## Runner Source

actions/runner v2.338.2 のソースから読み取れる実装です。

- ビルドコンテキスト：`src/Runner.Worker/ActionManager.cs` と `src/Runner.Worker/Handlers/ContainerActionHandler.cs` は、`docker build -t <タグ> -f <Dockerfile> <Dockerfile の親ディレクトリ>` を実行する。ビルドコンテキストは `Dockerfile` のあるディレクトリで、`image: Dockerfile` のときは `action.yml` のディレクトリと同じになる
- ビルドの時点：`ActionManager.cs` は、`uses:` が別のリポジトリを参照する Docker Action のイメージを、ジョブの準備の段階で `Build <リポジトリ>` という表示名の別のステップとしてビルドする。`uses: ./<パス>` で参照した同じリポジトリの Action は、`ContainerActionHandler.cs` がステップの中でビルドする。本調査で観測したのは後者で、post-entrypoint のステップでもビルドし直された。別のリポジトリを commit SHA で参照した前者では、ジョブのログの `docker build` は `Build <リポジトリ>` のステップの 1 回だけで、post-entrypoint のステップはビルドせずに `docker run` を始めた（2026/11/14 に観測）
- post の順序：`src/Runner.Worker/ActionRunner.cs` は、post を持つ Action のメインの処理を始めるときに、post のステップをジョブの `PostJobSteps`（スタック）に積む。`src/Runner.Worker/StepsRunner.cs` は、ジョブのステップを終えた後にスタックから取り出して動かす。post はメインの処理と逆の順で動き、メインの処理が動かなかったステップの post は積まれない
- マウント：`ContainerActionHandler.cs` は `/var/run/docker.sock`、`_temp` を `/github/runner_temp`、`_temp/_github_home` を `/github/home`、`_temp/_github_workflow` を `/github/workflow`、`_temp/_runner_file_commands` を `/github/file_commands`、ワークスペースを `/github/workspace` にマウントし、`HOME` を `/github/home` にする
- 環境変数：`ContainerActionHandler.cs` は、`ACTIONS_RUNTIME_URL` と `ACTIONS_RUNTIME_TOKEN` を常に、`ACTIONS_ID_TOKEN_REQUEST_URL` と `ACTIONS_ID_TOKEN_REQUEST_TOKEN` をジョブの接続情報に ID トークンの URL があるときだけ設定する。`ACTIONS_ID_TOKEN_REQUEST_TOKEN` の値は `ACTIONS_RUNTIME_TOKEN` と同じ接続情報のアクセストークンである
- 実行ユーザー：`src/Runner.Worker/Container/DockerCommandManager.cs` の `docker run` の組み立てに `--user` はない。実行ユーザーはイメージの `USER` で決まる
- `entrypoint` と `args`：`ContainerActionHandler.cs` は、`docker run` の `--entrypoint` に、メインでは `runs.entrypoint` を、post-entrypoint では `runs.post-entrypoint` を使う。`runs.args` は段階によらず同じように評価して `docker run` のコマンドの引数にし、`runs.args` がないときは、入力 `args` の値（未指定なら空）を引数にする
- キャンセル：`DockerCommandManager.cs` は、`docker run` を `--rm` 付きで、`--sig-proxy` を指定せず、`killProcessOnCancel: false` で起動する。この場合 `src/Runner.Sdk/ProcessInvoker.cs` は、キャンセルされた `docker run` のプロセスに SIGINT を送って 7.5 秒待ち、終わらなければ SIGTERM を送って 2.5 秒待ち、それでも終わらなければプロセスの木を強制終了する。`DockerCommandManager.cs` に `docker kill` と `docker stop` を送る処理はない
- キャンセルとファイルコマンド：`src/Runner.Worker/ActionRunner.cs` は、ハンドラーの実行を `try` に、ファイルコマンド（`GITHUB_OUTPUT`、`GITHUB_STATE` など）の処理を `finally` に置く。キャンセルか失敗で終わったステップでも、ファイルコマンドを処理する

## Action Files

`.github/actions/probe-docker/action.yml`

```yaml
name: Probe Docker
description: Probe how a Docker container action receives inputs and environment, what is mounted, and how state reaches the post-entrypoint
inputs:
  role-arn:
    description: Hyphenated input used to observe the INPUT_ variable name. Not a real role
    required: false
    default: arn:aws:iam::000000000000:role/probe
  fail:
    description: Set to "true" to make the main entrypoint exit 1
    required: false
    default: "false"
outputs:
  probe-value:
    description: Random base64 value, masked in the action, written as name=value
  probe-sha256:
    description: SHA-256 of probe-value, not masked
runs:
  using: docker
  image: Dockerfile
  entrypoint: /entrypoint.sh
  post-entrypoint: /post.sh
```

`.github/actions/probe-docker/Dockerfile`

```dockerfile
# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM docker.io/library/alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

# Uses busybox wget, sha256sum, base64, and mount only. No extra packages.
# No WORKDIR, as the GitHub documentation recommends.
COPY entrypoint.sh post.sh /
RUN chmod 0755 /entrypoint.sh /post.sh

ENTRYPOINT ["/entrypoint.sh"]
```

`probe-docker-nonroot` の `action.yml` は `name` と `description` だけが違います。
`Dockerfile` は `RUN chmod` の後に次の行を加えたもので、`entrypoint.sh` と `post.sh` は同じ内容の複製です。

```dockerfile
USER 65534:65534
```

`.github/actions/probe-docker/entrypoint.sh`

```sh
#!/bin/sh
# Probe for the Docker container action runtime.
# Prints only variable names, presence flags, paths, uid, and SHA-256 values.
# Never prints INPUT_ values, ACTIONS_* token values, the OIDC JWT, response headers, or a full environment dump.
# The values written to outputs and state are random strings only. They are not credentials.
set -eu

sha256() { printf '%s' "$1" | sha256sum | cut -d' ' -f1; }
present() { if [ -n "$(printenv "$1" || true)" ]; then echo "$1: present"; else echo "$1: absent"; fi; }

echo "== args"
echo "argument count: $#"

echo "== inputs: names of INPUT_* variables"
env | awk -F= '/^INPUT_/ { print $1 }' | sort
role_arn="$(printenv 'INPUT_ROLE-ARN' || true)"
echo "INPUT_ROLE-ARN length: ${#role_arn}, sha256: $(sha256 "${role_arn}")"
role_arn_underscore="$(printenv 'INPUT_ROLE_ARN' || true)"
echo "INPUT_ROLE_ARN length: ${#role_arn_underscore}"
fail="$(printenv INPUT_FAIL || true)"
echo "INPUT_FAIL: ${fail}"

echo "== oidc: presence of request variables (values are never printed)"
for name in ACTIONS_ID_TOKEN_REQUEST_URL ACTIONS_ID_TOKEN_REQUEST_TOKEN ACTIONS_RUNTIME_TOKEN ACTIONS_RUNTIME_URL ACTIONS_RESULTS_URL ACTIONS_CACHE_URL GITHUB_TOKEN; do
  present "${name}"
done

echo "== user"
echo "uid: $(id -u), gid: $(id -g)"
echo "pwd: $(pwd)"

echo "== mounts"
mount

echo "== docker socket"
if [ -S /var/run/docker.sock ]; then
  echo "/var/run/docker.sock: socket present"
  ls -ln /var/run/docker.sock
else
  echo "/var/run/docker.sock: absent"
fi

echo "== paths (value of the variable, and whether it exists in this container)"
for name in GITHUB_OUTPUT GITHUB_STATE GITHUB_ENV GITHUB_PATH GITHUB_STEP_SUMMARY RUNNER_TEMP RUNNER_TOOL_CACHE HOME GITHUB_WORKSPACE GITHUB_ACTION_PATH GITHUB_EVENT_PATH; do
  value="$(printenv "${name}" || true)"
  if [ -z "${value}" ]; then
    echo "${name}: unset"
  elif [ -e "${value}" ]; then
    echo "${name}: ${value} (exists)"
    ls -ldn "${value}"
  else
    echo "${name}: ${value} (missing)"
  fi
done
echo "file commands dir:"
ls -ldn "$(dirname "${GITHUB_OUTPUT}")"
echo "/github/runner_temp:"
ls -ldn /github/runner_temp 2>/dev/null || echo "absent"

echo "== workflow commands from stdout"
echo "::notice title=probe::notice emitted by uid $(id -u)"

echo "== mask, GITHUB_OUTPUT, GITHUB_STATE, GITHUB_ENV"
# 48 bytes encode to 64 characters.
value="$(head -c 48 /dev/urandom | base64 | tr -d '\n')"
state="$(head -c 48 /dev/urandom | base64 | tr -d '\n')"
echo "::add-mask::${value}"
echo "::add-mask::${state}"
value_sha="$(sha256 "${value}")"
state_sha="$(sha256 "${state}")"
echo "value length: ${#value}, sha256: ${value_sha}"
echo "state length: ${#state}, sha256: ${state_sha}"
echo "echo inside main (value): ${value}"
echo "echo inside main (state): ${state}"

if { echo "probe-value=${value}"; echo "probe-sha256=${value_sha}"; } >>"${GITHUB_OUTPUT}" 2>/dev/null; then
  echo "GITHUB_OUTPUT: writable"
else
  echo "GITHUB_OUTPUT: not writable"
fi
if { echo "probe_state=${state}"; echo "probe_state_sha256=${state_sha}"; } >>"${GITHUB_STATE}" 2>/dev/null; then
  echo "GITHUB_STATE: writable"
else
  echo "GITHUB_STATE: not writable"
fi
if echo "PROBE_ENV_SHA256=${value_sha}" >>"${GITHUB_ENV}" 2>/dev/null; then
  echo "GITHUB_ENV: writable"
else
  echo "GITHUB_ENV: not writable"
fi
if touch "${HOME}/probe-home" 2>/dev/null; then echo "HOME: writable"; else echo "HOME: not writable"; fi
if touch "${GITHUB_WORKSPACE}/probe-workspace" 2>/dev/null; then
  echo "GITHUB_WORKSPACE: writable"
  rm -f "${GITHUB_WORKSPACE}/probe-workspace"
else
  echo "GITHUB_WORKSPACE: not writable"
fi

echo "== oidc: request a JWT with audience sts.amazonaws.com (only status and sha256 are printed)"
if [ -n "$(printenv ACTIONS_ID_TOKEN_REQUEST_URL || true)" ] && [ -n "$(printenv ACTIONS_ID_TOKEN_REQUEST_TOKEN || true)" ]; then
  # The bearer token goes in a header, never in the URL or in any output.
  # -S writes response headers to stderr. They are captured and only the status code is printed.
  response_headers="$(wget -S -O /tmp/oidc.json \
    --header "Authorization: Bearer ${ACTIONS_ID_TOKEN_REQUEST_TOKEN}" \
    --header "Accept: application/json; api-version=2.0" \
    "${ACTIONS_ID_TOKEN_REQUEST_URL}&audience=sts.amazonaws.com" 2>&1 || true)"
  status="$(printf '%s\n' "${response_headers}" | sed -n 's/.*HTTP\/[0-9.]* \([0-9][0-9][0-9]\).*/\1/p' | head -n 1)"
  echo "oidc http status: ${status:-unknown}"
  if [ -z "${status}" ]; then
    # Prints only lines that cannot carry the bearer token or the JWT.
    printf '%s\n' "${response_headers}" | grep -i -E "^wget:|ssl" | head -n 5 || true
  fi
  jwt="$(sed -n 's/.*"value" *: *"\([^"]*\)".*/\1/p' /tmp/oidc.json 2>/dev/null || true)"
  rm -f /tmp/oidc.json
  if [ -n "${jwt}" ]; then
    echo "::add-mask::${jwt}"
    echo "oidc jwt length: ${#jwt}, dots: $(printf '%s' "${jwt}" | tr -cd '.' | wc -c | tr -d ' '), sha256: $(sha256 "${jwt}")"
  else
    echo "oidc jwt: not obtained"
  fi
else
  echo "oidc: request variables absent, skipped"
fi

if [ "${fail}" = "true" ]; then
  echo "::error title=probe::failing on purpose because input fail=true"
  exit 1
fi
echo "main: done"
```

`.github/actions/probe-docker/post.sh`

```sh
#!/bin/sh
# Post-entrypoint probe. Runs in a new container of the same image.
# Reads STATE_ variables written by the main entrypoint. Prints only names, uid, the masked echo, and SHA-256 values.
# The state value is a random string only. It is not a credential.
set -eu

sha256() { printf '%s' "$1" | sha256sum | cut -d' ' -f1; }
present() { if [ -n "$(printenv "$1" || true)" ]; then echo "$1: present"; else echo "$1: absent"; fi; }

echo "== post: uid $(id -u), pwd $(pwd)"
echo "== post: names of STATE_* variables"
env | awk -F= '/^STATE_/ { print $1 }' | sort
echo "== post: names of INPUT_* variables"
env | awk -F= '/^INPUT_/ { print $1 }' | sort
echo "== post: presence of oidc variables"
present ACTIONS_ID_TOKEN_REQUEST_URL
present ACTIONS_ID_TOKEN_REQUEST_TOKEN

state="$(printenv STATE_probe_state || true)"
expected="$(printenv STATE_probe_state_sha256 || true)"
echo "echo inside post (state): ${state}"
actual="$(sha256 "${state}")"
echo "state length: ${#state}, sha256: ${actual}, expected: ${expected:-none}"
if [ -n "${state}" ] && [ "${actual}" = "${expected}" ]; then echo "state: match"; else echo "state: MISMATCH or empty"; fi

if [ -e "${HOME}/probe-home" ]; then echo "HOME file from main: present"; else echo "HOME file from main: absent"; fi
if echo "probe-post=ran" >>"${GITHUB_OUTPUT}" 2>/dev/null; then echo "post GITHUB_OUTPUT: writable"; else echo "post GITHUB_OUTPUT: not writable"; fi
echo "::notice title=probe-post::post-entrypoint ran as uid $(id -u)"
```

`.github/actions/probe-docker-go/action.yml`

```yaml
name: Probe Docker Go
description: Measure the build time of a multi-stage image that compiles a Go program importing the AWS SDK, on golang alpine
inputs:
  role-arn:
    description: Hyphenated input used to observe the INPUT_ variable name. Not a real role
    required: false
    default: arn:aws:iam::000000000000:role/probe
outputs:
  go-uid:
    description: uid observed by the Go program
runs:
  using: docker
  image: Dockerfile
```

`.github/actions/probe-docker-go/Dockerfile`

```dockerfile
# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM docker.io/library/golang:1.27.3-alpine3.24@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS build

ENV CGO_ENABLED=0 GOFLAGS=-mod=readonly GOTOOLCHAIN=local
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN go build -trimpath -buildvcs=false -o /out/probe .

FROM scratch
# Only the CA certificates are copied, for HTTPS to GitHub and AWS.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/probe /probe
ENTRYPOINT ["/probe"]
```

`probe-docker-go-debian` の `Dockerfile` は、`FROM` を次の行にしたものです。
`action.yml` は `name` と `description` だけが違い、`.dockerignore`、`main.go`、`go.mod`、`go.sum` は同じ内容の複製です。

```dockerfile
FROM docker.io/library/golang:1.27.3-trixie@sha256:af1bfc2614ebb1dbd31af514b78740fd678850e76498762b425ecee4c5c6ea80 AS build
```

`.github/actions/probe-docker-go/.dockerignore`

```text
*
!go.mod
!go.sum
!main.go
```

`.github/actions/probe-docker-go/main.go`

```go
// Package main is a probe that measures the build time of an image importing the AWS SDK.
// It constructs clients without any network call and prints only variable names and the uid.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func main() {
	cfg := aws.Config{Region: "us-east-1"}
	stsClient := sts.NewFromConfig(cfg)
	lambdaClient := lambda.NewFromConfig(cfg)
	fmt.Printf("clients constructed: sts=%t lambda=%t\n", stsClient != nil, lambdaClient != nil)
	fmt.Printf("uid: %d, gid: %d\n", os.Getuid(), os.Getgid())

	var names []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "INPUT_") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Println(name)
	}

	f, err := os.OpenFile(os.Getenv("GITHUB_OUTPUT"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		fmt.Printf("GITHUB_OUTPUT: not writable: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	fmt.Fprintf(f, "go-uid=%d\n", os.Getuid())
	fmt.Println("GITHUB_OUTPUT: writable")
}
```

`.github/actions/probe-docker-go/go.mod`

```text
module probe

go 1.27.0

require (
	github.com/aws/aws-sdk-go-v2 v1.47.8
	github.com/aws/aws-sdk-go-v2/service/lambda v1.114.3
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.9
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.26 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.11 // indirect
	github.com/aws/smithy-go v1.28.8 // indirect
)
```

`go.sum` は `go mod tidy` で作りました。

`.github/actions/probe-runner-boundary/action.yml`

```yaml
name: Probe Runner Boundary
description: Probe what a root Docker container action can do through the mounted Docker socket, and leave random values in GITHUB_OUTPUT and GITHUB_STATE for later steps to look for
outputs:
  probe-value-sha256:
    description: SHA-256 of the random value written to GITHUB_OUTPUT as probe_value, not masked
  probe-state-sha256:
    description: SHA-256 of the random value written to GITHUB_STATE as probe_state, not masked
  output-file:
    description: Base name of the GITHUB_OUTPUT file of this step
  state-file:
    description: Base name of the GITHUB_STATE file of this step
runs:
  using: docker
  image: Dockerfile
  entrypoint: /entrypoint.sh
```

`.github/actions/probe-runner-boundary/Dockerfile`

```dockerfile
# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM docker.io/library/alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

# curl talks to the Docker Engine API over the unix socket, jq reads its responses,
# and su-exec runs the same requests as a non-root user.
RUN apk add --no-cache curl jq su-exec
COPY entrypoint.sh /
RUN chmod 0755 /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
```

ビルドで入ったのは curl 8.23.0、jq 1.8.2 と su-exec です。

`.github/actions/probe-runner-boundary/entrypoint.sh`

```sh
#!/bin/sh
# Probe for the boundary between a Docker container action and the Runner.
# Uses the mounted Docker socket only to read the Engine version, list containers,
# and start one container that mounts the host root read-only and reads /etc/os-release.
# Never inspects containers (their environment holds runtime tokens), never writes to the host,
# and never touches other containers or Runner processes.
# The values written to outputs and state are random strings only. They are not credentials.
set -eu

readonly SOCK=/var/run/docker.sock
BODY="$(mktemp)"
readonly BODY

sha256() { printf '%s' "$1" | sha256sum | cut -d' ' -f1; }

# api METHOD PATH [JSON] prints the HTTP status and leaves the response body in BODY.
api() {
  if [ $# -eq 3 ]; then
    curl -sS -o "${BODY}" -w '%{http_code}' --unix-socket "${SOCK}" -X "$1" \
      -H 'Content-Type: application/json' --data "$3" "http://localhost$2"
  else
    curl -sS -o "${BODY}" -w '%{http_code}' --unix-socket "${SOCK}" -X "$1" "http://localhost$2"
  fi
}

echo "== tools"
curl --version | head -n 1
jq --version

echo "== process"
echo "uid: $(id -u), gid: $(id -g), groups: $(id -G)"

echo "== socket"
if [ -S "${SOCK}" ]; then
  stat -c "${SOCK}: type=%F uid=%u gid=%g mode=%a" "${SOCK}"
else
  echo "${SOCK}: absent"
  exit 1
fi
sock_gid="$(stat -c %g "${SOCK}")"

echo "== GET /version"
status="$(api GET /version)"
echo "status: ${status}"
jq -r '"engine: \(.Version), api: \(.ApiVersion), os: \(.Os), kernel: \(.KernelVersion)"' "${BODY}"

echo "== GET /containers/json"
status="$(api GET /containers/json)"
echo "status: ${status}"
echo "running containers: $(jq 'length' "${BODY}")"
self_image="$(jq -r --arg h "$(hostname)" '.[] | select(.Id | startswith($h)) | .Image' "${BODY}")"
echo "this container found in the list: $([ -n "${self_image}" ] && echo yes || echo no)"
echo "images of running containers:"
jq -r '.[].Image' "${BODY}"

echo "== start a container that mounts the host root read-only"
# The container uses this action's own image, so nothing is pulled.
# It reads only /etc/os-release and checks that the Runner work directory exists.
script="grep -E '^(ID|VERSION_ID|PRETTY_NAME)=' /host/etc/os-release; if [ -d /host/home/runner/work ]; then echo 'host /home/runner/work: present'; else echo 'host /home/runner/work: absent'; fi; echo \"uid in the new container: \$(id -u)\""
create="$(jq -n --arg image "${self_image}" --arg script "${script}" \
  '{Image: $image, Entrypoint: ["/bin/sh", "-c"], Cmd: [$script], Tty: true, NetworkDisabled: true,
    HostConfig: {Binds: ["/:/host:ro"]}}')"
status="$(api POST /containers/create "${create}")"
echo "create status: ${status}"
id="$(jq -r '.Id // empty' "${BODY}")"
if [ -n "${id}" ]; then
  echo "start status: $(api POST "/containers/${id}/start")"
  echo "wait status: $(api POST "/containers/${id}/wait")"
  echo "exit code of the new container: $(jq -r '.StatusCode' "${BODY}")"
  echo "logs status: $(api GET "/containers/${id}/logs?stdout=1&stderr=1")"
  echo "-- output of the new container"
  tr -d '\r' < "${BODY}"
  echo "-- end of output"
  echo "remove status: $(api DELETE "/containers/${id}?force=true")"
else
  echo "create response message: $(jq -r '.message // empty' "${BODY}")"
fi

echo "== the same socket as a non-root user"
for user in "65534:65534" "65534:${sock_gid}"; do
  echo "-- su-exec ${user}"
  set +e
  result="$(su-exec "${user}" sh -c 'echo "uid: $(id -u), gid: $(id -g), groups: $(id -G)" >&2; curl -sS -o /dev/null -w "%{http_code}" --unix-socket /var/run/docker.sock http://localhost/version' 2>&1)"
  code=$?
  set -e
  echo "${result}"
  echo "curl exit code: ${code}"
done

echo "== runtime token and request token (values are never printed)"
if [ -n "${ACTIONS_RUNTIME_TOKEN:-}" ] && [ "${ACTIONS_RUNTIME_TOKEN}" = "${ACTIONS_ID_TOKEN_REQUEST_TOKEN:-}" ]; then
  echo "ACTIONS_RUNTIME_TOKEN equals ACTIONS_ID_TOKEN_REQUEST_TOKEN: yes"
else
  echo "ACTIONS_RUNTIME_TOKEN equals ACTIONS_ID_TOKEN_REQUEST_TOKEN: no"
fi
# A later run step compares its own request token with this hash and prints only the result.
sha256 "${ACTIONS_ID_TOKEN_REQUEST_TOKEN:-}" >/github/runner_temp/boundary-request-token.sha256

echo "== GITHUB_OUTPUT and GITHUB_STATE"
# 48 bytes encode to 64 characters.
value="$(head -c 48 /dev/urandom | base64 | tr -d '\n')"
state="$(head -c 48 /dev/urandom | base64 | tr -d '\n')"
echo "::add-mask::${value}"
echo "::add-mask::${state}"
value_sha="$(sha256 "${value}")"
state_sha="$(sha256 "${state}")"
echo "value length: ${#value}, sha256: ${value_sha}"
echo "state length: ${#state}, sha256: ${state_sha}"
{
  echo "probe_value=${value}"
  echo "probe-value-sha256=${value_sha}"
  echo "probe-state-sha256=${state_sha}"
  echo "output-file=$(basename "${GITHUB_OUTPUT}")"
  echo "state-file=$(basename "${GITHUB_STATE}")"
} >>"${GITHUB_OUTPUT}"
echo "probe_state=${state}" >>"${GITHUB_STATE}"
echo "GITHUB_OUTPUT: $(basename "${GITHUB_OUTPUT}")"
echo "GITHUB_STATE: $(basename "${GITHUB_STATE}")"
echo "main: done"
```

## Workflow File

```yaml
name: Docker Action Runtime
on:
  push:
    branches:
      - example-branch

permissions: {}

jobs:
  with-id-token:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      contents: read
      id-token: write
    steps:
      - name: Checkout
        uses: actions/checkout@d23441a48e516b6c34aea4fa41551a30e30af803 # v6.1.0
        with:
          persist-credentials: false

      - name: Runner info
        shell: bash
        run: |
          echo "image: ${ImageOS:-unknown} ${ImageVersion:-unknown}"
          docker version --format 'docker server {{.Server.Version}}, client {{.Client.Version}}'
          docker buildx version || true

      - name: Probe
        id: probe
        uses: ./.github/actions/probe-docker
        with:
          role-arn: arn:aws:iam::000000000000:role/probe

      # 値は乱数の文字列で、資格情報ではない
      - name: Echo output and compare hash
        shell: bash
        env:
          PROBE_VALUE: ${{ steps.probe.outputs.probe-value }}
          PROBE_SHA256: ${{ steps.probe.outputs.probe-sha256 }}
        run: |
          set -euo pipefail
          echo "probe-value: ${PROBE_VALUE}"
          actual="$(printf '%s' "${PROBE_VALUE}" | sha256sum | cut -d' ' -f1)"
          if [[ "${actual}" == "${PROBE_SHA256}" ]]; then
            echo "probe-value: match (length ${#PROBE_VALUE})"
          else
            echo "probe-value: MISMATCH (length ${#PROBE_VALUE})"
          fi
          echo "role-arn sha256 (caller side): $(printf '%s' 'arn:aws:iam::000000000000:role/probe' | sha256sum | cut -d' ' -f1)"
          echo "PROBE_ENV_SHA256 via GITHUB_ENV: ${PROBE_ENV_SHA256:-unset}"

  without-id-token:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      contents: read
    steps:
      - name: Checkout
        uses: actions/checkout@d23441a48e516b6c34aea4fa41551a30e30af803 # v6.1.0
        with:
          persist-credentials: false

      - name: Probe
        id: probe
        uses: ./.github/actions/probe-docker

      - name: Echo output and compare hash
        shell: bash
        env:
          PROBE_VALUE: ${{ steps.probe.outputs.probe-value }}
          PROBE_SHA256: ${{ steps.probe.outputs.probe-sha256 }}
        run: |
          set -euo pipefail
          echo "probe-value: ${PROBE_VALUE}"
          actual="$(printf '%s' "${PROBE_VALUE}" | sha256sum | cut -d' ' -f1)"
          if [[ "${actual}" == "${PROBE_SHA256}" ]]; then
            echo "probe-value: match (length ${#PROBE_VALUE})"
          else
            echo "probe-value: MISMATCH (length ${#PROBE_VALUE})"
          fi

  main-fails:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      contents: read
    steps:
      - name: Checkout
        uses: actions/checkout@d23441a48e516b6c34aea4fa41551a30e30af803 # v6.1.0
        with:
          persist-credentials: false

      - name: Probe (main exits 1)
        id: probe
        continue-on-error: true
        uses: ./.github/actions/probe-docker
        with:
          fail: "true"

      - name: Report outcome
        if: always()
        shell: bash
        env:
          OUTCOME: ${{ steps.probe.outcome }}
          PROBE_SHA256: ${{ steps.probe.outputs.probe-sha256 }}
        run: |
          echo "probe outcome: ${OUTCOME}"
          echo "probe-sha256 length after failure: ${#PROBE_SHA256}"

  nonroot:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      contents: read
      id-token: write
    steps:
      - name: Checkout
        uses: actions/checkout@d23441a48e516b6c34aea4fa41551a30e30af803 # v6.1.0
        with:
          persist-credentials: false

      - name: Probe as uid 65534
        id: probe
        continue-on-error: true
        uses: ./.github/actions/probe-docker-nonroot

      # 値は乱数の文字列で、資格情報ではない
      - name: Echo output and compare hash
        if: always()
        shell: bash
        env:
          OUTCOME: ${{ steps.probe.outcome }}
          PROBE_VALUE: ${{ steps.probe.outputs.probe-value }}
          PROBE_SHA256: ${{ steps.probe.outputs.probe-sha256 }}
        run: |
          echo "probe outcome: ${OUTCOME}"
          echo "probe-value: ${PROBE_VALUE}"
          echo "probe-value length: ${#PROBE_VALUE}, probe-sha256 length: ${#PROBE_SHA256}"
          actual="$(printf '%s' "${PROBE_VALUE}" | sha256sum | cut -d' ' -f1)"
          if [[ -n "${PROBE_VALUE}" && "${actual}" == "${PROBE_SHA256}" ]]; then
            echo "probe-value: match"
          else
            echo "probe-value: MISMATCH or empty"
          fi
          echo "PROBE_ENV_SHA256 via GITHUB_ENV: ${PROBE_ENV_SHA256:-unset}"

  go-build-alpine:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    permissions:
      contents: read
    steps:
      - name: Checkout
        uses: actions/checkout@d23441a48e516b6c34aea4fa41551a30e30af803 # v6.1.0
        with:
          persist-credentials: false

      - name: Probe (Go, golang alpine)
        id: probe
        uses: ./.github/actions/probe-docker-go

      - name: Echo output
        shell: bash
        env:
          GO_UID: ${{ steps.probe.outputs.go-uid }}
        run: |
          echo "go-uid: ${GO_UID}"
          docker image ls --format '{{.Repository}}:{{.Tag}} {{.Size}}'

  go-build-debian:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    permissions:
      contents: read
    steps:
      - name: Checkout
        uses: actions/checkout@d23441a48e516b6c34aea4fa41551a30e30af803 # v6.1.0
        with:
          persist-credentials: false

      - name: Probe (Go, golang debian)
        id: probe
        uses: ./.github/actions/probe-docker-go-debian

      - name: Echo output
        shell: bash
        env:
          GO_UID: ${{ steps.probe.outputs.go-uid }}
        run: |
          echo "go-uid: ${GO_UID}"
          docker image ls --format '{{.Repository}}:{{.Tag}} {{.Size}}'
```

[Runner Boundary](#runner-boundary) の調査用の Action を呼んだワークフローです。

```yaml
name: Runner Boundary
on:
  push:
    branches:
      - example-branch

permissions: {}

defaults:
  run:
    shell: bash

jobs:
  boundary:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      contents: read
      id-token: write
    steps:
      - name: Checkout
        uses: actions/checkout@d23441a48e516b6c34aea4fa41551a30e30af803 # v6.1.0
        with:
          persist-credentials: false

      - name: Runner info
        run: |
          echo "image: ${ImageOS:-unknown} ${ImageVersion:-unknown}"
          echo "runner user: $(id)"
          docker version --format 'docker server {{.Server.Version}}, client {{.Client.Version}}'
          ls -ln /var/run/docker.sock
          getent group docker || true
          if sudo -n true 2> /dev/null; then echo "sudo -n: allowed"; else echo "sudo -n: denied"; fi
          echo "RUNNER_TEMP: ${RUNNER_TEMP}"
          echo "HOME: ${HOME}"

      # 前のステップが残したプロセスが、後のステップのファイルコマンドのファイルを読めるかを見る
      - name: Start a background watcher
        run: |
          mkdir -p "${RUNNER_TEMP}/boundary-watch"
          # shellcheck disable=SC2016
          nohup bash -c '
            end=$((SECONDS + 300))
            while ((SECONDS < end)); do
              for f in "${RUNNER_TEMP}"/_runner_file_commands/*; do
                [[ -s "${f}" ]] && cp -f "${f}" "${RUNNER_TEMP}/boundary-watch/" 2> /dev/null
              done
              sleep 0.1
            done
          ' > /dev/null 2>&1 &
          echo "watcher pid: $!"

      - name: Probe
        id: probe
        uses: ./.github/actions/probe-runner-boundary

      # 値は乱数の文字列で、資格情報ではない。値は出さず、長さと SHA-256 の一致だけを出す
      - name: Read file commands of the probe step
        env:
          VALUE_SHA256: ${{ steps.probe.outputs.probe-value-sha256 }}
          STATE_SHA256: ${{ steps.probe.outputs.probe-state-sha256 }}
          OUTPUT_FILE: ${{ steps.probe.outputs.output-file }}
          STATE_FILE: ${{ steps.probe.outputs.state-file }}
        run: |
          set -euo pipefail
          dir="${RUNNER_TEMP}/_runner_file_commands"
          echo "== ${dir}"
          ls -ln "${dir}"
          echo "this step GITHUB_OUTPUT: $(basename "${GITHUB_OUTPUT}")"

          # check LABEL FILE KEY EXPECTED
          check() {
            local label="$1" file="$2" key="$3" expected="$4" value actual
            if [[ ! -e "${file}" ]]; then
              echo "${label}: file absent"
              return
            fi
            if [[ ! -r "${file}" ]]; then
              echo "${label}: file not readable"
              return
            fi
            value="$(sed -n "s/^${key}=//p" "${file}" | head -n 1)"
            actual="$(printf '%s' "${value}" | sha256sum | cut -d' ' -f1)"
            if [[ -n "${value}" && "${actual}" == "${expected}" ]]; then
              echo "${label}: readable, match (length ${#value})"
            else
              echo "${label}: readable, MISMATCH or empty (length ${#value})"
            fi
          }

          echo "== files of the probe step, read after the step"
          check "GITHUB_OUTPUT ${OUTPUT_FILE}" "${dir}/${OUTPUT_FILE}" probe_value "${VALUE_SHA256}"
          check "GITHUB_STATE ${STATE_FILE}" "${dir}/${STATE_FILE}" probe_state "${STATE_SHA256}"

          echo "== copies made by the background watcher during the probe step"
          ls -ln "${RUNNER_TEMP}/boundary-watch"
          check "watcher copy of GITHUB_OUTPUT" "${RUNNER_TEMP}/boundary-watch/${OUTPUT_FILE}" probe_value "${VALUE_SHA256}"
          check "watcher copy of GITHUB_STATE" "${RUNNER_TEMP}/boundary-watch/${STATE_FILE}" probe_state "${STATE_SHA256}"
          if pgrep -f 'boundary-watch' > /dev/null; then echo "watcher: still running"; else echo "watcher: not running"; fi

      # 値は出さない。有無と、OIDC JWT の要求の HTTP ステータスだけを出す
      - name: Request variables in a run step
        run: |
          set -euo pipefail
          for name in ACTIONS_ID_TOKEN_REQUEST_URL ACTIONS_ID_TOKEN_REQUEST_TOKEN ACTIONS_RUNTIME_TOKEN ACTIONS_RUNTIME_URL ACTIONS_RESULTS_URL; do
            if [[ -n "${!name:-}" ]]; then echo "${name}: present"; else echo "${name}: absent"; fi
          done
          if [[ -n "${ACTIONS_ID_TOKEN_REQUEST_TOKEN:-}" ]]; then
            status="$(curl -sS -o /dev/null -w '%{http_code}' \
              -H "Authorization: Bearer ${ACTIONS_ID_TOKEN_REQUEST_TOKEN}" \
              "${ACTIONS_ID_TOKEN_REQUEST_URL}&audience=sts.amazonaws.com")"
            echo "oidc jwt request from a run step: HTTP status ${status}"
            actual="$(printf '%s' "${ACTIONS_ID_TOKEN_REQUEST_TOKEN}" | sha256sum | cut -d' ' -f1)"
            if [[ "${actual}" == "$(cat "${RUNNER_TEMP}/boundary-request-token.sha256")" ]]; then
              echo "request token equals the one in the probe container: yes"
            else
              echo "request token equals the one in the probe container: no"
            fi
          fi

      # 名前だけを出し、値は出さない
      - name: Environment of the Runner processes
        run: |
          set -uo pipefail
          for name in Runner.Listener Runner.Worker; do
            for pid in $(pgrep -f "bin/${name}"); do
              echo "== ${name} pid ${pid} (owner $(stat -c %U "/proc/${pid}"))"
              if names="$(tr '\0' '\n' < "/proc/${pid}/environ" 2> /dev/null | cut -d= -f1 | sort)"; then
                echo "environ readable without sudo: yes"
              else
                echo "environ readable without sudo: no"
                if names="$(sudo -n cat "/proc/${pid}/environ" 2> /dev/null | tr '\0' '\n' | cut -d= -f1 | sort)"; then
                  echo "environ readable with sudo -n: yes"
                else
                  echo "environ readable with sudo -n: no"
                  continue
                fi
              fi
              echo "variable count: $(grep -c . <<< "${names}")"
              echo "names:"
              echo "${names}"
            done
          done
```

[Files Left by the GATES Action](#files-left-by-the-gates-action) のワークフローです。
接続情報は E2E テストと同じ Secrets と Variables から渡し、`_e2e-reusable-credential-leak.yml` は GATES のリポジトリの E2E テストの写しです。

```yaml
name: Runner Boundary GATES
on:
  push:
    branches:
      - example-branch

permissions: {}

# 同じ Lambda を呼ぶ E2E テストを重ねない
concurrency:
  group: gates-e2e
  cancel-in-progress: false

defaults:
  run:
    shell: bash

jobs:
  issue:
    name: Issue and list files
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      id-token: write
      contents: read
    steps:
      # RUNNER_TEMP（Action の HOME の _github_home を含む）と、ワークスペースを除く HOME のファイルの一覧を取る。
      # 種別、大きさ、パスだけを出し、内容は出さない
      - name: List files before the action
        run: |
          set -euo pipefail
          marker="${RUNNER_TEMP}/../boundary-marker"
          touch "${marker}"
          find "${RUNNER_TEMP}" "${HOME}" -path "${HOME}/work" -prune -o -type f -printf '%s %p\n' | sort -k2 > "${RUNNER_TEMP}/../boundary-before.txt"
          echo "files before: $(wc -l < "${RUNNER_TEMP}/../boundary-before.txt")"
          echo "== RUNNER_TEMP before"
          find "${RUNNER_TEMP}" -printf '%y %s %p\n' | sort -k3

      - name: Issue token
        id: gates
        uses: tmknom/gates/.github/actions/issue-token@<commit SHA>
        with:
          repositories: ${{ vars.GATES_TARGET_PRIMARY }}
          permissions: contents:read
          role-arn: arn:aws:iam::${{ secrets.GATES_AWS_ACCOUNT_ID }}:role/${{ vars.GATES_ROLE_NAME }}
          function-name: ${{ vars.GATES_FUNCTION_NAME }}
          aws-region: ${{ vars.GATES_AWS_REGION }}
          audience: ${{ vars.GATES_AUDIENCE }}

      # トークンの値は出さない。トークンを含むファイルの名前と、差分のファイルの名前と大きさだけを出す
      - name: List files after the action
        env:
          TOKEN: ${{ steps.gates.outputs.token }}
          PRIMARY: ${{ vars.GATES_TARGET_PRIMARY }}
        run: |
          set -euo pipefail
          echo "token length: ${#TOKEN}"
          status="$(GH_TOKEN="${TOKEN}" gh api --include "repos/${PRIMARY}/contents/README.md" 2> /dev/null | head -n 1 | awk '{print $2}' || true)"
          echo "token works: README.md ${status:-unknown}"

          find "${RUNNER_TEMP}" "${HOME}" -path "${HOME}/work" -prune -o -type f -printf '%s %p\n' | sort -k2 > "${RUNNER_TEMP}/../boundary-after.txt"
          echo "files after: $(wc -l < "${RUNNER_TEMP}/../boundary-after.txt")"
          echo "== added or changed files (size and path)"
          # diff は差分があると 1 で終わるため、pipefail で失敗扱いにしないよう結果をファイルに置いてから読む
          diff "${RUNNER_TEMP}/../boundary-before.txt" "${RUNNER_TEMP}/../boundary-after.txt" > "${RUNNER_TEMP}/../boundary-diff.txt" || true
          grep -E '^[<>]' "${RUNNER_TEMP}/../boundary-diff.txt" || echo "no difference"
          echo "== RUNNER_TEMP after"
          find "${RUNNER_TEMP}" -printf '%y %s %p\n' | sort -k3

          echo "== files under RUNNER_TEMP that contain the token"
          grep -rlsF -e "${TOKEN}" "${RUNNER_TEMP}" || echo "none"
          echo "== files under HOME (except the work directory) changed during the action that contain the token"
          find "${HOME}" -path "${HOME}/work" -prune -o -type f -newer "${RUNNER_TEMP}/../boundary-marker" -print0 \
            | xargs -0 -r grep -lsF -e "${TOKEN}" || echo "none"

  credential-leak:
    name: Credential leak
    needs: issue
    if: ${{ always() }}
    permissions:
      actions: read
    uses: ./.github/workflows/_e2e-reusable-credential-leak.yml
```

## References

- [GitHub — Dockerfile support for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/dockerfile-support)
- [GitHub — Creating a Docker container action](https://docs.github.com/en/actions/tutorials/use-containerized-services/create-a-docker-container-action)
- [GitHub — About custom actions](https://docs.github.com/en/actions/concepts/workflows-and-actions/custom-actions)
- [GitHub — Metadata syntax reference](https://docs.github.com/en/actions/reference/workflows-and-actions/metadata-syntax)
- [GitHub — Workflow commands for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)
- [GitHub — Secure use reference](https://docs.github.com/en/actions/reference/security/secure-use)
- [GitHub — GitHub-hosted runners reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)
- [actions/runner v2.338.2 — ContainerActionHandler.cs](https://github.com/actions/runner/blob/v2.338.2/src/Runner.Worker/Handlers/ContainerActionHandler.cs)
- [actions/runner v2.338.2 — DockerCommandManager.cs](https://github.com/actions/runner/blob/v2.338.2/src/Runner.Worker/Container/DockerCommandManager.cs)
- [actions/runner v2.338.2 — ActionManager.cs](https://github.com/actions/runner/blob/v2.338.2/src/Runner.Worker/ActionManager.cs)
- [actions/runner-images — README](https://github.com/actions/runner-images/blob/main/README.md)
- [actions/runner-images — Ubuntu 24.04](https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md)
- [actions/runner-images — Ubuntu 26.04](https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2604-Readme.md)
