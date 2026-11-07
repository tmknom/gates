# GitHub OIDC Custom Audience

本ドキュメントでは、GitHub Actions の OIDC JWT を Audience を指定して取得し、その結果を記述します。
GitHub Actions の実環境で実行し、ログへ出力した値を掲載しています。

- 調査日: **2026/11/07**
- 検証環境: 個人アカウントのプライベートリポジトリ

## Request

`ACTIONS_ID_TOKEN_REQUEST_URL` に `&audience=<URI エンコードした値>` を付けた GET で、Audience を指定して JWT を取得しました。
起動イベントは workflow_dispatch で、ワークフローはオーナー `octocat` のリポジトリ `example-repo` にあります。

## Issued JWT

URL でない文字列を指定して取得した JWT 2 件について、値は出力せず、型だけを出力しました。
2 件に指定した Audience の値と、2 件が同じ値だったかは記録していません。
出力には、取得した JWT の Header と Payload を `jq` で復号し、Header のキーと `alg`、`aud` の型、時刻と ID の Claim の型だけを選ぶステップを使いました。
このステップの定義は記録していないため、[Workflow File](#workflow-file) には含めていません。

- `aud` は指定した値そのものの JSON 文字列で、配列ではなかった
- `exp`、`iat`、`nbf` は JSON の数値、`actor_id`、`repository_owner_id`、`repository_id`、`run_id`、`run_attempt` は JSON の文字列だった
- Header のキーは `alg`（`RS256`）、`kid`、`typ`、`x5t` だった

## Refused Audience

Audience の値によっては、GitHub が JWT を発行しません。
拒否された要求の応答は `400` で、本文は次の形でした。

```json
{"source":"actions-run-service","statusCode":400,"errorMessage":"Can't issue ID_TOKEN for audience 'https://github.com/octocat/example-other-repo'"}
```

`example-repo` のワークフローから、値を変えて取得を試した結果は次のとおりです。
発行された要求の応答は出力していません。

| 指定した Audience | 結果 |
|---|---|
| `https://github.com/octocat`（呼び出し元のオーナーの URL） | 発行 |
| `https://github.com/octocat/example-repo`（呼び出し元のリポジトリの URL） | 発行 |
| `https://github.com` | 発行 |
| `https://github.com/octocat/example-other-repo`（同じオーナーの別のリポジトリの URL） | `400` |
| `https://github.com/octocat/example-other-repo/`（末尾にスラッシュ） | `400` |
| `https://GitHub.com/octocat/example-other-repo`（ホスト名に大文字） | `400` |
| `https://github.com/another-owner`（別のオーナーの URL） | `400` |
| `https://api.github.com/octocat/example-other-repo` | `400` |
| `https://octocat.github.io/example-other-repo`、`https://another-owner.github.io/example-other-repo` | 発行 |
| `https://example.com/octocat/example-other-repo`、`https://gates.example.com` | 発行 |
| `urn:octocat:gates`、`api://octocat-gates`、`octocat-gates` | 発行 |
| `sts.amazonaws.com` | 発行 |

試した github.com の URL のうち、発行されたのは呼び出し元のオーナーかリポジトリを指すものと `https://github.com` だけでした。
api.github.com は、別のリポジトリを指す URL だけを試しました。
github.io では、呼び出し元と異なるオーナーのサブドメインも発行されました。
この制限は、GitHub の公式資料には記載がありません。

## Workflow File

発行を拒否される Audience の調査に使用したワークフローファイルは次のとおりです。
発行された要求の応答は JWT を含むため出力せず、ステータスとエラー文言だけを出力します。

```yaml
name: OIDC Audience
on:
  workflow_dispatch:
jobs:
  probe:
    runs-on: ubuntu-latest
    timeout-minutes: 5
    permissions:
      id-token: write
    steps:
      - name: Request ID tokens per audience
        shell: bash
        run: |
          set -euo pipefail
          echo "::add-mask::${ACTIONS_ID_TOKEN_REQUEST_TOKEN}"
          audiences=(
            "https://github.com/${GITHUB_REPOSITORY_OWNER}"
            "https://github.com/${GITHUB_REPOSITORY_OWNER}/example-other-repo"
            # 表の値を並べる
          )
          for audience in "${audiences[@]}"; do
            encoded="$(jq -rn --arg v "${audience}" '$v | @uri')"
            body="$(curl --silent --show-error --write-out '\n%{http_code}' \
              -H "Authorization: Bearer ${ACTIONS_ID_TOKEN_REQUEST_TOKEN}" \
              -H "Accept: application/json" \
              "${ACTIONS_ID_TOKEN_REQUEST_URL}&audience=${encoded}")"
            status="${body##*$'\n'}"
            if [[ "${status}" == "200" ]]; then
              echo "${status} ${audience}"
            else
              message="$(jq -r '.errorMessage // empty' <<< "${body%$'\n'*}" || true)"
              echo "${status} ${audience} : ${message}"
            fi
            unset body
          done
```
