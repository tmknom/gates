# GitHub Repository Identity

本ドキュメントでは、個人アカウントの private リポジトリを改名し、削除して同じ名前で作り直したときの Repository ID と、OIDC JWT と REST API がリポジトリを示す値を記述します。
Installation の対象のリポジトリを改名した後に、新しい名前と旧名で Installation Access Token の発行を要求した結果も記述します。

- 調査日: **2026/11/21**
- 要求: GitHub Actions のワークフローで OIDC JWT を取得し、`GITHUB_TOKEN` で `GET https://api.github.com/repos/{owner}/{repo}` と `GET https://api.github.com/repositories/{repository_id}` を呼んだ。発行は、GitHub Actions のワークフローからデプロイした GATES に Docker Action で要求した。GATES は `POST https://api.github.com/app/installations/{installation_id}/access_tokens` に対象のリポジトリ名と `{"contents":"read"}` を指定する
- 検証環境: 個人アカウント。2026 年 7 月 15 日より後に作成した private リポジトリだけを使った。検証用 GitHub App の Installation は `Only select repositories` で、対象は private リポジトリ 2 件

オーナー名、リポジトリ名、ID は置き換えています。
置き換えた後も、名前の大文字小文字、値が等しいか異なるか、ID の桁数（10 桁）は元の値のとおりです。
OIDC JWT の署名部、トークン、発行の応答本文は記録していません。

| 置き換えた名前 | ID | 役割 |
|---|---|---|
| `octocat` | `583231` | オーナー（`User`） |
| `Probe-Repo` | `1234500003` | 改名と作り直しに使った、Installation の対象でない使い捨てのリポジトリ |
| `target-repo` | `1234500002` | 改名した Installation の対象 |
| `caller-repo` | `1234500001` | 発行を要求したワークフローのリポジトリ |

## 取得方法

使い捨てのリポジトリには、push で起動する次のワークフローを置きました。
改名の前と後に push して、それぞれの実行のログを記録しました。

```yaml
name: Repository Identity Probe
on:
  push:
  workflow_dispatch:

permissions: {}

defaults:
  run:
    shell: bash

jobs:
  probe:
    runs-on: ubuntu-latest
    timeout-minutes: 5
    permissions:
      id-token: write
      contents: read
    steps:
      # JWT の署名部は出さず、Payload のうちリポジトリを指す Claim だけを出す
      - name: Show JWT claims
        run: |
          set -euo pipefail
          echo "::add-mask::${ACTIONS_ID_TOKEN_REQUEST_TOKEN}"
          token="$(curl --silent --show-error --fail \
            -H "Authorization: Bearer ${ACTIONS_ID_TOKEN_REQUEST_TOKEN}" \
            -H "Accept: application/json; api-version=2.0" \
            "${ACTIONS_ID_TOKEN_REQUEST_URL}" | jq -er '.value')"
          echo "::add-mask::${token}"
          payload="$(cut -d '.' -f 2 <<< "${token}" | tr '_-' '/+')"
          while (( ${#payload} % 4 != 0 )); do payload="${payload}="; done
          base64 -d <<< "${payload}" | jq '{sub, repository, repository_id, repository_owner, repository_owner_id}'

      - name: Show GitHub context
        env:
          REPOSITORY: ${{ github.repository }}
          REPOSITORY_ID: ${{ github.repository_id }}
        run: |
          echo "github.repository=${REPOSITORY}"
          echo "github.repository_id=${REPOSITORY_ID}"

      # 応答の本文は出さず、id と名前だけを出す
      - name: Show REST API repository
        env:
          GH_TOKEN: ${{ github.token }}
          REPOSITORY_ID: ${{ github.repository_id }}
        run: |
          set -euo pipefail
          gh api "repos/${GITHUB_REPOSITORY}" --jq '{id, name, full_name}'
          gh api "repositories/${REPOSITORY_ID}" --jq '{id, name, full_name}'
```

改名、削除、作成は、運用者が `gh repo rename`、`gh repo create`、GitHub の画面で行いました。
改名と作り直しの前後の `id` は、運用者の端末の `gh api repos/{owner}/{repo}` でも記録しました。
`gh api` はリダイレクトをたどるため、旧名への要求に `301` が返ったかは記録していません。

Installation の対象を改名している間は、`caller-repo` のワークフローから、新しい名前と旧名を 1 回ずつ要求しました。
発行に成功したら、トークンで `GET /installation/repositories` と `GET /repos/{owner}/{repo}` を呼び、`id`、`name`、`full_name` だけを出しました。
発行に失敗したら、GATES の監査記録の `result` と `reason` を記録しました。
発行の応答本文は GATES が記録しないため、GitHub の応答は、応答コードと、発行したトークンで呼んだ API の応答から読みました。

## 改名と作り直しでの Repository ID

使い捨てのリポジトリを `Probe-Repo` として作り、`Probe-Repo-Renamed` に改名し、削除して `Probe-Repo-Renamed` を作り直しました。

| 順 | 操作 | `GET /repos/{owner}/{repo}` の `id` と `name` | `GET /repositories/{id}` |
|---|---|---|---|
| 1 | `Probe-Repo` を作る | `1234500003`、`Probe-Repo` | `1234500003` の `Probe-Repo` |
| 2 | `Probe-Repo-Renamed` に改名する | `1234500003`、`Probe-Repo-Renamed` | `1234500003` の `Probe-Repo-Renamed` |
| 3 | 改名した後に、旧名の `Probe-Repo` を指定する | `1234500003`、`Probe-Repo-Renamed` | |
| 4 | `Probe-Repo-Renamed` を削除する | | `1234500003` は `404` |
| 5 | `Probe-Repo-Renamed` を同じ名前で作り直す | `1234500004`、`Probe-Repo-Renamed` | `1234500003` は `404` のまま |

改名の前後で `id` は変わらず、作り直したリポジトリの `id` は削除したリポジトリと異なりました。
改名した後は、旧名を指定した `GET /repos/{owner}/{repo}` も、改名したリポジトリを返しました。

Installation の対象の `target-repo` も、`target-repo-renamed` に改名した後と、元の名前に戻した後で、`id` は `1234500002` のままでした。

## OIDC JWT と REST API の値

使い捨てのリポジトリの改名の前と後に、同じワークフローが出した値です。

| 値 | 改名の前 | 改名の後 |
|---|---|---|
| JWT の `sub` | `repo:octocat@583231/Probe-Repo@1234500003:ref:refs/heads/main` | `repo:octocat@583231/Probe-Repo-Renamed@1234500003:ref:refs/heads/main` |
| JWT の `repository` | `octocat/Probe-Repo` | `octocat/Probe-Repo-Renamed` |
| JWT の `repository_id` | `"1234500003"` | `"1234500003"` |
| JWT の `repository_owner`、`repository_owner_id` | `octocat`、`"583231"` | `octocat`、`"583231"` |
| `github.repository`、`github.repository_id` | `octocat/Probe-Repo`、`1234500003` | `octocat/Probe-Repo-Renamed`、`1234500003` |
| REST API の `id` | `1234500003` | `1234500003` |
| REST API の `name` | `Probe-Repo` | `Probe-Repo-Renamed` |
| REST API の `full_name` | `octocat/Probe-Repo` | `octocat/Probe-Repo-Renamed` |

- **ID**：JWT の `repository_id` は JSON の文字列、REST API の `id` は JSON の数値で、10 進の表記は同じだった
- **名前**：`sub` のリポジトリ部の名前、`repository` のスラッシュより後の部分、REST API の `name` は、大文字小文字を含めて同じだった
- **改名の後**：改名の後に発行された JWT の `sub` と `repository` は、新しい名前を示した。`sub` の Repository ID は変わらなかった

発行を要求した `caller-repo` の JWT（GATES の監査記録の Claims Set）でも、`repository_id` は運用者の端末で取得した REST API の `id` と同じ値で、`repository` は REST API の `full_name` と同じでした。

## 改名した Installation の対象への発行

Installation の対象の `target-repo` を `target-repo-renamed` に改名した後、`caller-repo` のワークフローから要求しました。

| 要求の対象 | GATES の結果 | GitHub の応答 | トークンから見える対象 |
|---|---|---|---|
| `octocat/target-repo-renamed` | 成功 | `201` | 1 件。`id` は `1234500002`、`full_name` は `octocat/target-repo-renamed` |
| `octocat/target-repo` | `github_issuance_rejected` | `422` | なし（発行されない） |

新しい名前の要求では、GATES の Output の `repositories` は `octocat/target-repo-renamed` で、`permissions` は `contents:read,metadata:read` でした。
トークンで呼んだ `GET /repos/octocat/target-repo-renamed` も、`id` が `1234500002` でした。

旧名の要求の監査記録の `reason` は次のとおりで、GitHub の応答コードは `422` でした。

```text
issue: github issuance rejected: status 422, github request id "<omitted>"
```

旧名で要求した発行は、改名したリポジトリに読み替えた `201` を返しませんでした。
旧名を指定した `GET /repos/{owner}/{repo}` が改名したリポジトリを返したこと（[改名と作り直しでの Repository ID](#改名と作り直しでの-repository-id)）と、発行 API の扱いは異なります。
`422` の本文は記録していないため、存在しない名前を要求したときの `422` と本文が同じかは確かめていません。

元の名前に戻した後、`caller-repo` の既存の E2E テストの成功の事例を実行しました。
`contents:read` だけを要求する事例は、`target-repo` を含む要求を含めてすべて成功しました。
`contents:read` のほかの権限を要求する事例は、GATES に設定した発行してよい権限が `contents:read` だけのため、どれも `authorization_denied` でした。
