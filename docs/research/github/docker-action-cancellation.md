# GitHub Docker Action Failure and Cancellation

本ドキュメントでは、GATES の Docker Action（`issue-token`）を、失敗するジョブとキャンセルするジョブから呼んだときの、post-entrypoint の実行とキャンセルのシグナルの届き方を記述します。

- 調査日：**2026/11/10**
- 検証環境：個人アカウントのプライベートリポジトリ。`runs-on: ubuntu-latest`（Ubuntu 26.04、Runner 2.338.2）
- 起動：作業ブランチへの push
- ワークフロー：本リポジトリの [_e2e-failure.yml](../../../.github/workflows/_e2e-failure.yml) と [_e2e-cancel.yml](../../../.github/workflows/_e2e-cancel.yml) の `on:` に push のトリガーを加えたもの。Action は別のリポジトリとして commit SHA で参照した

Action が発行したトークンは、どれも同じジョブの中で失効させ、失効の HTTP ステータスが `204` であることを確かめました。
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

SIGINT から 7.5 秒後の SIGTERM と、`docker run` のプロセスの強制終了の後のコンテナの扱いは、確かめていません。
Action のメインの処理はどの場合も 7.5 秒より前に終わったためです。

## References

- [GitHub — Workflow cancellation reference](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-cancellation)
- [GitHub — Workflow syntax for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)
- [GitHub — REST API endpoints for workflow runs](https://docs.github.com/en/rest/actions/workflow-runs)
