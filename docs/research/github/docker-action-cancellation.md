# GitHub Docker Action Failure and Cancellation

本ドキュメントでは、GATES の Docker Action（`issue-token`）を、失敗するジョブとキャンセルするジョブから呼んだときの、post-entrypoint の実行とキャンセルのシグナルの届き方を記述します。
あわせて、`docker run` のプロセスを強制終了した後のコンテナと、post-entrypoint が失効させた後のトークンを記述します。

- 調査日：**2026/11/10**（[Killing the Docker Run Process](#killing-the-docker-run-process) と [Revoked Token after the Post-entrypoint](#revoked-token-after-the-post-entrypoint) は **2026/11/14**）
- 検証環境：個人アカウントのプライベートリポジトリ。`runs-on: ubuntu-latest`（Ubuntu 26.04、Runner 2.338.2）
- 起動：作業ブランチへの push
- ワークフロー：本リポジトリの [_e2e-failure.yml](../../../.github/workflows/_e2e-failure.yml) と [_e2e-cancel.yml](../../../.github/workflows/_e2e-cancel.yml) の `on:` に push のトリガーを加えたもの。Action は別のリポジトリとして commit SHA で参照した

Action が発行したトークンは、どれも同じジョブの中で失効させ、失効の HTTP ステータスが `204` であることを確かめました。
例外は、強制終了の最初の実行でコンテナの出力を読めなかった 1 件で、失効の結果は確かめられていません。
ジョブのログ全体に `ghs_`、`eyJ`、`ASIA`、`FwoG`、`IQoJ` の文字列がないことを、実行の中で確かめました。

## Post-entrypoint in Failed Jobs

メインのステップに `continue-on-error` を付けずに失敗させたジョブでも、post-entrypoint は動きました。
入力の検証で失敗させたジョブのステップの結果は次のとおりで、ジョブの結果は `failure` でした。

| ステップ | 結果 |
|---|---|
| `Issue token`（メイン） | `failure` |
| 後続のステップ | `skipped` |
| `Post Issue token`（post-entrypoint） | `success`。`docker run` の `--entrypoint` は `"/revoke-token"` |

メインが `GITHUB_STATE` に書く前に失敗したため、post-entrypoint は何も出力しませんでした。
`continue-on-error: true` を付けて失敗させた 10 のジョブでも、post-entrypoint は同じく動き、何も出力しませんでした。

メインが成功し、後続のステップが `exit 1` で失敗したジョブでも、post-entrypoint は動き、`revocation succeeded: HTTP status 204` を出しました。

## Cancellation of a Workflow Run

GitHub は 1 つのジョブだけをキャンセルする API を持たないため、`POST /repos/{owner}/{repo}/actions/runs/{run_id}/cancel` でワークフロー実行をキャンセルしました。
メインが成功し、後続のステップの `sleep` が動いている間にキャンセルしたジョブでは、次のとおりでした。

- 後続のステップは `##[error]The operation was canceled.` で終わり、結果は `cancelled` だった
- その直後に post-entrypoint が動き、`revocation succeeded: HTTP status 204` を出した。ステップの結果は `success` だった
- ジョブの結果は `cancelled` だった
- `if: always()` を付けて `needs` で待つジョブは、キャンセルの後に動いた

キャンセルの API を呼んでから、Runner が後続のステップを終えるまでの時間は次のとおりでした。
どちらも、API を呼んだ時刻と、後続のステップの `The operation was canceled.` の行の時刻の差です。

| 実行 | API を呼んだ時刻 | ステップを終えた時刻 | 差 |
|---|---|---|---|
| 1 回目 | 08:04:16.6 前後（`sleep 3` の後に `curl`） | 08:04:29.9 | 約 13 秒 |
| 2 回目 | 08:08:13.6 | 08:08:26.0 | 約 12.4 秒 |

1 回目の実行では、API を呼んだ後に Action のメインの処理を始めたジョブが、キャンセルを受ける前にメインの処理を終えていました。
Invoke の開始を示す行は 08:04:15、Action のステップは約 1.5 秒で終わり、キャンセルは次のステップに届きました。

## SIGINT to the Docker Run Process

[Workflow cancellation reference](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-cancellation) は、キャンセルするステップのコンテナの Action に対して、Runner が `docker` のプロセスに SIGINT を送るとしています。
ワークフロー実行のキャンセルでは、メインの処理の最中にシグナルを届けられないため、同じ Runner の上で切り離して起動したプロセスから、Action のステップの `docker run --entrypoint /issue-token` のプロセスに、そのプロセスが現れてから指定の時間の後に SIGINT を送りました。

| 待ち | SIGINT の時点 | Action の出力 | ステップの `outcome` | Output の `token` |
|---|---|---|---|---|
| 0 ms | Invoke の前 | `##[error]the job was canceled`。Invoke の行と失効の行はない | `failure` | 空 |
| 500 ms | Invoke の前 | 同上 | `failure` | 空 |
| 1000 ms | Invoke の開始の直後 | Invoke の行、約 3.4 秒後に `revocation succeeded: HTTP status 204`、`##[error]the job was canceled` | `failure` | 空 |
| 1500 ms | Invoke の開始の後 | Invoke の行、`revocation succeeded: HTTP status 204`、`##[error]the job was canceled` | `failure` | 空 |
| 2000 ms | Invoke の開始の後 | 同上 | `failure` | 空 |
| 2500 ms | Action の終了の後（送る前に `docker run` のプロセスが終わっていた） | Invoke の行だけで、Action は成功した。post-entrypoint が `revocation succeeded: HTTP status 204` を出した | `success` | 390 文字 |

`docker run` は受けた SIGINT をコンテナへ中継し、コンテナの中の Action が受けた時点に応じて終えました。
1000 ms のジョブでは、SIGINT は Invoke の開始を示す行の 26 ms 後に届き、Action は Invoke を打ち切らずに約 3.4 秒後の応答を待ってから、受け取ったトークンを失効させました。
どの場合も、post-entrypoint は `::warning::` を出しませんでした。

SIGINT から 7.5 秒後の SIGTERM は、確かめていません。
Action のメインの処理はどの場合も 7.5 秒より前に終わったためです。

## Killing the Docker Run Process

Runner は、SIGINT から 10 秒で終わらない `docker run` のプロセスを強制終了します（actions/runner v2.338.2 の `src/Runner.Sdk/ProcessInvoker.cs`）。
Invoke の最中にこれが起きた場合を、同じ Runner の上で切り離したプロセスから、`docker run` のプロセスに SIGINT を送った約 57 ms 後に SIGKILL を送って起こしました。
メインの処理を 10 秒まで遅らせる手段は、本番のコードに試験のための分岐を加えない限りないため、強制終了までの時間を縮めています。
SIGTERM は、Action が SIGINT と同じ受け口で扱うため送っていません。

- 調査日：**2026/11/14**
- 検証環境：個人アカウントのプライベートリポジトリ。`runs-on: ubuntu-latest`（Ubuntu 26.04、Runner 2.338.2）
- Action：別のリポジトリとして commit SHA で参照した

`docker run` は `--rm` 付きで起動されるため、コンテナは終わるとすぐ削除され、その後は出力も終了コードも読めません。
そのため、`docker run` のプロセスが現れたら、シグナルを送る前に `docker logs -f` と `docker wait` でコンテナを追い、出力のうち資格情報を含まない決まった行（Invoke、失効、`::error::`、`::warning::` の行）だけを残しました。

| 待ち | SIGINT の時点 | SIGKILL の後 | コンテナの出力 | コンテナの終了 | ステップ |
|---|---|---|---|---|---|
| 1000 ms | Invoke の開始の後 | `docker run` のプロセスはなく、コンテナは `running` | Invoke の行、`revocation succeeded: HTTP status 204`、`::error::the job was canceled` | SIGKILL の約 0.9 秒後に終了コード 1。その直後に削除された | `failure`、Output は空 |
| 1500 ms | Invoke の開始の後 | 同上 | 同上 | SIGKILL の約 0.7 秒後に終了コード 1 | `failure`、Output は空 |
| 2000 ms | 送らなかった。Action が先に成功して終わった | － | Invoke の行だけ | 終了コード 0 | `success`。post-entrypoint が `204` で失効させた |

`docker run` のプロセスを強制終了した後も、コンテナは動き続け、Invoke の応答を待ってからトークンを失効させました。
Runner のステップは SIGKILL の時点で `failure` として終わり、コンテナの出力はステップのログに残りませんでした。
キャンセルされたコンテナは `GITHUB_STATE` に書かないため、post-entrypoint は何もしませんでした。

追う処理をシグナルの後に始めた最初の実行でも、1000 ms と 1500 ms の事例は同じ結果でした。
一方で 2000 ms の事例は、SIGINT と SIGKILL を送った約 0.2 秒後にはコンテナが削除されていて、出力を読めませんでした。
Invoke の開始の行から SIGINT までは約 1.2 秒で、ステップは `failure`、Output は空、post-entrypoint の `docker run` に `STATE_token` はありませんでした。
Runner は失敗で終わったステップのファイルコマンドも処理するため（actions/runner v2.338.2 の `src/Runner.Worker/ActionRunner.cs` は、その処理を `finally` に置く）、コンテナは `GITHUB_STATE` に書かずに終わったと考えられます。
このジョブの GATES の監査記録の時刻は、SIGINT を送った時刻の 41 ms 前でした。
監査記録は Lambda が応答を返す直前に書かれ、応答はそこから Runner まで届く必要があるため、子は応答を受け取る前に SIGINT を受けた可能性が高いと考えられます。
その場合、子は失敗時の失効を試みて終えます。
ただし、2 つの時刻は Lambda と Runner の別々の時計によるもので、失効の結果は確かめられていません。

ジョブは、`_e2e-cancel.yml` の `signal-main` のジョブと同じく、Action のステップの前に次のステップを置き、Action のステップに `continue-on-error: true` を付けました。
後続のステップが `kill.log` に `done` の行が現れるまで待ってから、その内容を出します。
待ちは `matrix` の `delay-ms` で変えました。

```bash
setsid nohup bash -c '
  umask 077
  log="${RUNNER_TEMP}/kill.log"
  now() { date -u +%H:%M:%S.%3N; }
  for _ in $(seq 1 6000); do
    line="$(pgrep -a -f -n "^/usr/bin/docker run .*--entrypoint /issue-token " || true)"
    [[ -n "${line}" ]] && break
    sleep 0.05
  done
  if [[ -z "${line}" ]]; then
    echo "docker run did not appear" >> "${log}"
    echo "done" >> "${log}"
    exit 1
  fi
  pid="${line%% *}"
  name="$(grep -o -E -- "--name [^ ]+" <<< "${line}" | cut -d " " -f 2)"
  echo "docker run appeared at $(now)" >> "${log}"
  # --rm のコンテナは終わるとすぐ削除されるため、シグナルの前に追跡を始める
  for _ in $(seq 1 200); do
    docker inspect "${name}" > /dev/null 2>&1 && break
    sleep 0.01
  done
  docker wait "${name}" > "${RUNNER_TEMP}/kill-exit-code" 2>&1 &
  waiter=$!
  {
    docker logs -f "${name}" 2>&1 \
      | grep -E "^(Invoking the Lambda function at |revocation (succeeded|failed)|::error::|::warning::)" \
      | sed "s/^/container: /" > "${RUNNER_TEMP}/kill-container.log" || true
  } &
  follower=$!
  echo "following the container from $(now)" >> "${log}"
  sleep "$(awk -v ms="${DELAY_MS}" "BEGIN { print ms / 1000 }")"
  kill -INT "${pid}" && echo "SIGINT sent at $(now)" >> "${log}"
  sleep 0.05
  kill -KILL "${pid}" && echo "SIGKILL sent at $(now)" >> "${log}"
  sleep 0.2
  echo "docker run process after SIGKILL: $(ps -o stat= -p "${pid}" || echo gone)" >> "${log}"
  echo "container after SIGKILL: $(docker inspect --format "{{.State.Status}} AutoRemove={{.HostConfig.AutoRemove}}" "${name}" 2>&1 || true)" >> "${log}"
  wait "${waiter}" || true
  wait "${follower}" || true
  cat "${RUNNER_TEMP}/kill-container.log" >> "${log}"
  echo "container exited with $(cat "${RUNNER_TEMP}/kill-exit-code") at $(now)" >> "${log}"
  echo "done" >> "${log}"
' > /dev/null 2>&1 < /dev/null &
```

残す行の先頭に `container: ` を付けるのは、後続のステップが出したときに、Runner がワークフローコマンドとして扱わないようにするためです。
コンテナの出力はトークンのマスクの行を含むため、ファイルには決まった行だけを書きます。

## Revoked Token after the Post-entrypoint

post-entrypoint が失効させた後のトークンが使えないことを、post-entrypoint の後に動く post から確かめました。

- 調査日：**2026/11/14**
- 検証環境：上と同じ
- ワークフロー：本リポジトリの [_e2e-success.yml](../../../.github/workflows/_e2e-success.yml) の `on:` に push のトリガーを加えたもの。`Revoked token` のジョブは、Docker Action のステップより前に、post を持つ試験用の JavaScript の Action（[_e2e-revoked-token](../../../.github/actions/_e2e-revoked-token/action.yml)）を置く

`Revoked token` のジョブのステップは、次の順に動き、どれも `success` でした。
番号はジョブの API の `number` で、7〜9 は API の一覧にありませんでした。

| 番号 | ステップ | ログ |
|---|---|---|
| 1、2 | `Set up job`、`Build <Action のリポジトリ>` | Docker Action のイメージのビルド。ジョブのログの `docker build` はこの 1 回だけ |
| 3、4 | `Checkout`、`Check revoked token`（試験用の Action） | トークンを書くファイルの場所 |
| 5 | `Issue token`（Docker Action） | 発行の成功 |
| 6 | `Save token` | トークンで要求した対象の `README.md` を読み `200`。ファイルに書いた |
| 10 | `Post Issue token`（post-entrypoint） | ビルドせずに `docker run` を始め、`revocation succeeded: HTTP status 204` |
| 11 | `Post Check revoked token`（試験用の Action の post） | 同じ `README.md` を読み `revoked token read README.md: HTTP status 401`、`token file removed` |
| 12 | `Post Checkout` | － |

post はメインの処理と逆の順で動き、試験用の Action の post は post-entrypoint の約 0.3 秒後に、失効したトークンで `401` を得ました。
post-entrypoint の失効の要求から `401` までの間に、失効が効かない時間は観測されませんでした。

## References

- [GitHub — Workflow cancellation reference](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-cancellation)
- [GitHub — Workflow syntax for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)
- [GitHub — REST API endpoints for workflow runs](https://docs.github.com/en/rest/actions/workflow-runs)
