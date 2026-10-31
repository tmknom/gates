# GitHub OIDC JWT

本ドキュメントでは、GitHub OIDC が扱う JWT Claim を実際に取得し、その結果を記述します。
GitHub Actions の実環境で実行し、ログへ出力した値を掲載しています。

- 調査日: **2026/10/30〜2026/10/31**
- 検証環境: 個人アカウントのプライベートリポジトリ

## Push Event

### 起動条件

- Trigger Event: push
- Reusable Workflows: なし
- GitHub Environment: なし

### Payload

```json
{
  "actor": "octocat",
  "actor_id": "583231",
  "aud": "https://github.com/octocat",
  "base_ref": "",
  "check_run_id": "123919064426",
  "event_name": "push",
  "exp": 1793340472,
  "head_ref": "",
  "iat": 1793340172,
  "iss": "https://token.actions.githubusercontent.com",
  "job_workflow_ref": "octocat/example-repo/.github/workflows/oidc.yml@refs/heads/test-branch",
  "job_workflow_sha": "cc87f748bf3d7040c38cf0523d5f315b4a2d7e89",
  "jti": "dec56e62-02aa-482d-a954-d0aaaef107fc",
  "nbf": 1793339872,
  "ref": "refs/heads/test-branch",
  "ref_protected": "false",
  "ref_type": "branch",
  "repository": "octocat/example-repo",
  "repository_id": "1234567890",
  "repository_owner": "octocat",
  "repository_owner_id": "583231",
  "repository_visibility": "private",
  "run_attempt": "1",
  "run_id": "41011783574",
  "run_number": "1",
  "runner_environment": "github-hosted",
  "sha": "cc87f748bf3d7040c38cf0523d5f315b4a2d7e89",
  "sub": "repo:octocat@583231/example-repo@1234567890:ref:refs/heads/test-branch",
  "workflow": "OIDC",
  "workflow_ref": "octocat/example-repo/.github/workflows/oidc.yml@refs/heads/test-branch",
  "workflow_sha": "cc87f748bf3d7040c38cf0523d5f315b4a2d7e89"
}
```

## Pull Request Event

### 起動条件

- Trigger Event: pull_request
- Reusable Workflows: なし
- GitHub Environment: なし

### Payload

```json
{
  "actor": "octocat",
  "actor_id": "583231",
  "aud": "https://github.com/octocat",
  "base_ref": "main",
  "check_run_id": "123921407524",
  "event_name": "pull_request",
  "exp": 1793340937,
  "head_ref": "test-branch",
  "iat": 1793340637,
  "iss": "https://token.actions.githubusercontent.com",
  "job_workflow_ref": "octocat/example-repo/.github/workflows/oidc.yml@refs/pull/1/merge",
  "job_workflow_sha": "7d9beb10a3689ea68d09c3e236a9ca3d2e321b62",
  "jti": "37ef094d-44fa-46f0-ad9f-ea55842787a2",
  "nbf": 1793340337,
  "ref": "refs/pull/1/merge",
  "ref_protected": "false",
  "ref_type": "branch",
  "repository": "octocat/example-repo",
  "repository_id": "1234567890",
  "repository_owner": "octocat",
  "repository_owner_id": "583231",
  "repository_visibility": "private",
  "run_attempt": "1",
  "run_id": "41012535428",
  "run_number": "2",
  "runner_environment": "github-hosted",
  "sha": "7d9beb10a3689ea68d09c3e236a9ca3d2e321b62",
  "sub": "repo:octocat@583231/example-repo@1234567890:pull_request",
  "workflow": "OIDC",
  "workflow_ref": "octocat/example-repo/.github/workflows/oidc.yml@refs/pull/1/merge",
  "workflow_sha": "7d9beb10a3689ea68d09c3e236a9ca3d2e321b62"
}
```

## Workflow Dispatch Event

### 起動条件

- Trigger Event: workflow_dispatch
- Reusable Workflows: なし
- GitHub Environment: なし

### Payload

```json
{
  "actor": "octocat",
  "actor_id": "583231",
  "aud": "https://github.com/octocat",
  "base_ref": "",
  "check_run_id": "124444658783",
  "event_name": "workflow_dispatch",
  "exp": 1793422055,
  "head_ref": "",
  "iat": 1793421755,
  "iss": "https://token.actions.githubusercontent.com",
  "job_workflow_ref": "octocat/example-repo/.github/workflows/oidc.yml@refs/heads/main",
  "job_workflow_sha": "557ca50fa5a17294d2950cb3992f14dc4d03090e",
  "jti": "7c5220f0-fde0-4d9d-998f-7c3c6ea07a88",
  "nbf": 1793421455,
  "ref": "refs/heads/main",
  "ref_protected": "false",
  "ref_type": "branch",
  "repository": "octocat/example-repo",
  "repository_id": "1234567890",
  "repository_owner": "octocat",
  "repository_owner_id": "583231",
  "repository_visibility": "private",
  "run_attempt": "1",
  "run_id": "41169659213",
  "run_number": "5",
  "runner_environment": "github-hosted",
  "sha": "557ca50fa5a17294d2950cb3992f14dc4d03090e",
  "sub": "repo:octocat@583231/example-repo@1234567890:ref:refs/heads/main",
  "workflow": "OIDC",
  "workflow_ref": "octocat/example-repo/.github/workflows/oidc.yml@refs/heads/main",
  "workflow_sha": "557ca50fa5a17294d2950cb3992f14dc4d03090e"
}
```

## Absolute Reusable Workflows

### 起動条件

- Trigger Event: push
- Reusable Workflows: `<owner>/<repo>/.github/workflows/reusable.yml@<commit-sha>`
- GitHub Environment: なし

### Payload

```json
{
  "actor": "octocat",
  "actor_id": "583231",
  "aud": "https://github.com/octocat",
  "base_ref": "",
  "check_run_id": "123938755311",
  "event_name": "push",
  "exp": 1793344300,
  "head_ref": "",
  "iat": 1793344000,
  "iss": "https://token.actions.githubusercontent.com",
  "job_workflow_ref": "octocat/example-repo/.github/workflows/reusable.yml@557ca50fa5a17294d2950cb3992f14dc4d03090e",
  "job_workflow_sha": "557ca50fa5a17294d2950cb3992f14dc4d03090e",
  "jti": "7b5d75a7-98a0-4007-99e6-b6776981bd92",
  "nbf": 1793343700,
  "ref": "refs/heads/test-branch",
  "ref_protected": "false",
  "ref_type": "branch",
  "repository": "octocat/example-repo",
  "repository_id": "1234567890",
  "repository_owner": "octocat",
  "repository_owner_id": "583231",
  "repository_visibility": "private",
  "run_attempt": "1",
  "run_id": "41018050776",
  "run_number": "4",
  "runner_environment": "github-hosted",
  "sha": "00661965ed8327bb0aba156ea4ef5bdc171e6fd8",
  "sub": "repo:octocat@583231/example-repo@1234567890:ref:refs/heads/test-branch",
  "workflow": "OIDC",
  "workflow_ref": "octocat/example-repo/.github/workflows/oidc.yml@refs/heads/test-branch",
  "workflow_sha": "00661965ed8327bb0aba156ea4ef5bdc171e6fd8"
}
```

## Relative Reusable Workflows

### 起動条件

- Trigger Event: pull_request
- Reusable Workflows: `./.github/workflows/reusable.yml@<commit-sha>`
- GitHub Environment: なし

### Payload

```json
{
  "actor": "octocat",
  "actor_id": "583231",
  "aud": "https://github.com/octocat",
  "base_ref": "main",
  "check_run_id": "123936636925",
  "event_name": "pull_request",
  "exp": 1793343920,
  "head_ref": "test-branch",
  "iat": 1793343620,
  "iss": "https://token.actions.githubusercontent.com",
  "job_workflow_ref": "octocat/example-repo/.github/workflows/reusable.yml@refs/pull/2/merge",
  "job_workflow_sha": "92fb7d2bf90d2034a72f9bb1e885c96ab5a211bd",
  "jti": "ca9c4c4b-2a70-4bdc-a031-5ec7843a7fdb",
  "nbf": 1793343320,
  "ref": "refs/pull/2/merge",
  "ref_protected": "false",
  "ref_type": "branch",
  "repository": "octocat/example-repo",
  "repository_id": "1234567890",
  "repository_owner": "octocat",
  "repository_owner_id": "583231",
  "repository_visibility": "private",
  "run_attempt": "1",
  "run_id": "41017362820",
  "run_number": "3",
  "runner_environment": "github-hosted",
  "sha": "92fb7d2bf90d2034a72f9bb1e885c96ab5a211bd",
  "sub": "repo:octocat@583231/example-repo@1234567890:pull_request",
  "workflow": "OIDC",
  "workflow_ref": "octocat/example-repo/.github/workflows/oidc.yml@refs/pull/2/merge",
  "workflow_sha": "92fb7d2bf90d2034a72f9bb1e885c96ab5a211bd"
}
```

## GitHub Environment

### 起動条件

- Trigger Event: pull_request
- Reusable Workflows: なし
- GitHub Environment: `testing`

### Payload

```json
{
  "actor": "octocat",
  "actor_id": "583231",
  "aud": "https://github.com/octocat",
  "base_ref": "main",
  "check_run_id": "123936636603",
  "environment": "testing",
  "environment_node_id": "EN_kwDOSZYC0s8AAAABKgfUQA",
  "event_name": "pull_request",
  "exp": 1793343910,
  "head_ref": "test-branch",
  "iat": 1793343610,
  "iss": "https://token.actions.githubusercontent.com",
  "job_workflow_ref": "octocat/example-repo/.github/workflows/oidc.yml@refs/pull/2/merge",
  "job_workflow_sha": "92fb7d2bf90d2034a72f9bb1e885c96ab5a211bd",
  "jti": "6a56642a-879a-4777-b024-4daed3bf14ee",
  "nbf": 1793343310,
  "ref": "refs/pull/2/merge",
  "ref_protected": "false",
  "ref_type": "branch",
  "repository": "octocat/example-repo",
  "repository_id": "1234567890",
  "repository_owner": "octocat",
  "repository_owner_id": "583231",
  "repository_visibility": "private",
  "run_attempt": "1",
  "run_id": "41017362820",
  "run_number": "3",
  "runner_environment": "github-hosted",
  "sha": "92fb7d2bf90d2034a72f9bb1e885c96ab5a211bd",
  "sub": "repo:octocat@583231/example-repo@1234567890:environment:testing",
  "workflow": "OIDC",
  "workflow_ref": "octocat/example-repo/.github/workflows/oidc.yml@refs/pull/2/merge",
  "workflow_sha": "92fb7d2bf90d2034a72f9bb1e885c96ab5a211bd"
}
```

## Header

Payload と異なり、Header はすべて共通でした。

```json
{
  "alg": "RS256",
  "kid": "38826b17-6a30-5f9b-b169-8beb8202f723",
  "typ": "JWT",
  "x5t": "ykNaY4qM_ta4k2TgZOCEYLkcYlA"
}
```

## Workflow File

GitHub OIDC の JWT Claim の取得に使用したワークフローファイルは次のとおりです。

```yaml
name: OIDC
on:
  workflow_dispatch:
  pull_request:
  push:
jobs:
  standard:
    runs-on: ubuntu-slim
    timeout-minutes: 5
    # environment: testing
    permissions:
      id-token: write
    steps:
      - name: Request ID Token
        id: id-token
        run: |
          echo "::add-mask::${ACTIONS_ID_TOKEN_REQUEST_TOKEN}"
          token=$(curl --silent --show-error --fail \
            -H "Authorization: Bearer ${ACTIONS_ID_TOKEN_REQUEST_TOKEN}" \
            -H "Accept: application/json; api-version=2.0" \
            -H "Content-Type: application/json" \
            -d "{}" "${ACTIONS_ID_TOKEN_REQUEST_URL}" | \
            jq -er '.value')
          echo "::add-mask::${token}"
          echo "header=$(cut -d '.' -f 1 <<<"${token}")" >> "${GITHUB_OUTPUT}"
          echo "payload=$(cut -d '.' -f 2 <<<"${token}")" >> "${GITHUB_OUTPUT}"
        shell: bash

      - name: Show JWT
        env:
          HEADER: ${{ steps.id-token.outputs.header }}
          PAYLOAD: ${{ steps.id-token.outputs.payload }}
        run: |
          decode() {
            local base64_encoded
            base64_encoded="$(tr '_-' '/+' <<<"$1")"
            local remainder=$(( ${#base64_encoded} % 4 ))
            if [[ "${remainder}" != 0 ]]; then
              base64_encoded="${base64_encoded}$(printf '=%.0s' $(seq 1 $(( 4 - remainder ))))"
            fi
            base64 -d <<<"${base64_encoded}"
          }
          jq -n \
            --argjson header "$(decode "${HEADER}")" \
            --argjson payload "$(decode "${PAYLOAD}")" \
            '{header: $header, payload: $payload}'
        shell: bash

  reusable:
    permissions:
      id-token: write
    uses: ./.github/workflows/reusable.yml # standard ジョブとほぼ同じ
```
