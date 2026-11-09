# GitHub Docker Action Runtime

本ドキュメントでは、リポジトリ内の Docker Action（`runs.using: docker`、`image: Dockerfile`）を GitHub Actions の実環境で動かしたときの挙動を記述します。
調べたのは、入力の受け取り方、OIDC JWT の取得、コンテナのマウント、実行ユーザー、Output とマスク、post-entrypoint への状態の受け渡し、イメージのビルドの時間です。
観測と合わせて、actions/runner v2.338.2 のソースから読み取れる実装も記録します。

- 調査日：**2026/11/09**
- 検証環境：個人アカウントのプライベートリポジトリ。`runs-on: ubuntu-latest`
- 起動：作業ブランチへの push

オーナー名は `octocat`、リポジトリ名は `example-repo`、ブランチ名は `example-branch` に置き換えています。
Output、`GITHUB_STATE`、`GITHUB_ENV` に書いた値は、コンテナの中で `/dev/urandom` から作った乱数の文字列と、その SHA-256 のハッシュ値だけです。
OIDC JWT と、その要求に使うトークン（`ACTIONS_ID_TOKEN_REQUEST_TOKEN`）の値は、ログにも本ドキュメントにも出していません。
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

- **Docker ソケット**：`/var/run/docker.sock` はソケットとして存在し、所有者は uid 0、グループは gid 113、権限は `srw-rw----` だった。権限から見て、root で動くコンテナはこのソケットに書き込める。ソケット経由で Docker を操作することは試していない
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

## Runner Source

actions/runner v2.338.2 のソースから読み取れる実装です。

- ビルドコンテキスト：`src/Runner.Worker/ActionManager.cs` と `src/Runner.Worker/Handlers/ContainerActionHandler.cs` は、`docker build -t <タグ> -f <Dockerfile> <Dockerfile の親ディレクトリ>` を実行する。ビルドコンテキストは `Dockerfile` のあるディレクトリで、`image: Dockerfile` のときは `action.yml` のディレクトリと同じになる
- ビルドの時点：`ActionManager.cs` は、`uses:` が別のリポジトリを参照する Docker Action のイメージを、ジョブの準備の段階で `Build <リポジトリ>` という表示名の別のステップとしてビルドする。`uses: ./<パス>` で参照した同じリポジトリの Action は、`ContainerActionHandler.cs` がステップの中でビルドする。本調査で観測したのは後者で、post-entrypoint のステップでもビルドし直された。別のリポジトリを参照した場合の post-entrypoint でビルドし直すかは確かめていない
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

## References

- [GitHub — Dockerfile support for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/dockerfile-support)
- [GitHub — Creating a Docker container action](https://docs.github.com/en/actions/tutorials/use-containerized-services/create-a-docker-container-action)
- [GitHub — About custom actions](https://docs.github.com/en/actions/concepts/workflows-and-actions/custom-actions)
- [GitHub — Metadata syntax reference](https://docs.github.com/en/actions/reference/workflows-and-actions/metadata-syntax)
- [GitHub — Workflow commands for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)
- [actions/runner v2.338.2 — ContainerActionHandler.cs](https://github.com/actions/runner/blob/v2.338.2/src/Runner.Worker/Handlers/ContainerActionHandler.cs)
- [actions/runner v2.338.2 — DockerCommandManager.cs](https://github.com/actions/runner/blob/v2.338.2/src/Runner.Worker/Container/DockerCommandManager.cs)
- [actions/runner v2.338.2 — ActionManager.cs](https://github.com/actions/runner/blob/v2.338.2/src/Runner.Worker/ActionManager.cs)
- [actions/runner-images — README](https://github.com/actions/runner-images/blob/main/README.md)
- [actions/runner-images — Ubuntu 24.04](https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md)
- [actions/runner-images — Ubuntu 26.04](https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2604-Readme.md)
