# AWS STS Legacy Subject

本ドキュメントでは、名前だけの旧形式の `sub` を持つ GitHub Actions の OIDC JWT で、GATES の GitHub Actions 用 IAM Role の引き受けを試みた結果を記述します。
GitHub Actions の実環境で実行し、ログへ出力した値を掲載しています。

- 調査日: **2026/11/08**
- 要求: `aws sts assume-role-with-web-identity`（AWS CLI v2）
- 検証環境: 個人アカウントのプライベートリポジトリ。2026-07-15 より前に作られ、immutable `sub` にオプトインしていない

オーナー名は `octocat`、リポジトリ名は `example-legacy-repo` と `example-repo`、ブランチ名は `example-branch`、AWS アカウント ID は `123456789012`、Owner ID は `583231`、Repository ID は `1234567890` に置き換えています。
OIDC JWT の値は記録していません。

## Repository Setting

リポジトリの `GET /repos/{owner}/{repo}/actions/oidc/customization/sub` の応答は次のとおりでした。

```json
{"use_default":true,"use_immutable_subject":false,"sub_claim_prefix":"repo:octocat/example-legacy-repo"}
```

2026-07-15 以降に作られたリポジトリは immutable `sub` を自動で使い、`include_claim_keys` でカスタマイズしても ID を外せないと、GitHub の公式資料が記述しています（[GitHub Constraints](../../spec/github-constraints.md#32-immutable-subject-とカスタマイズ)）。
このため、旧形式の `sub` の JWT は、同日より前に作られたリポジトリでだけ取得できます。

## Trust Policy

引き受けを試みた IAM Role の Trust Policy の条件は次のとおりです。

```json
{
  "StringEquals": { "token.actions.githubusercontent.com:aud": "sts.amazonaws.com" },
  "StringLike": { "token.actions.githubusercontent.com:sub": "repo:octocat@583231/*" }
}
```

## Result

Audience に `sts.amazonaws.com` を指定して取得した JWT の `sub` と `aud` は次のとおりでした。

```json
{
  "sub": "repo:octocat/example-legacy-repo:ref:refs/heads/example-branch",
  "aud": "sts.amazonaws.com"
}
```

この JWT での引き受けは拒否され、AWS CLI の終了コードは `254` でした。

```text
aws: [ERROR]: An error occurred (AccessDenied) when calling the AssumeRoleWithWebIdentity operation: Not authorized to perform sts:AssumeRoleWithWebIdentity
```

エラー文言は、Trust Policy のどの条件が満たされなかったかを示しません。
同じ時刻以降、GATES の関数のロググループに行はありませんでした。

## CloudTrail

`AssumeRoleWithWebIdentity` は管理イベントで、拒否された引き受けも `ap-northeast-1` のイベント履歴に残りました。
記録の主な項目は次のとおりで、送信元 IP アドレスと各 ID は省いています。

```json
{
  "userIdentity": {
    "type": "WebIdentityUser",
    "principalId": "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com:sts.amazonaws.com:repo:octocat/example-legacy-repo:ref:refs/heads/example-branch",
    "userName": "repo:octocat/example-legacy-repo:ref:refs/heads/example-branch",
    "identityProvider": "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"
  },
  "eventSource": "sts.amazonaws.com",
  "eventName": "AssumeRoleWithWebIdentity",
  "errorCode": "AccessDenied",
  "errorMessage": "Not authorized to perform sts:AssumeRoleWithWebIdentity",
  "requestParameters": null,
  "responseElements": null,
  "resources": [
    {"type": "AWS::IAM::OIDCProvider", "ARN": "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"},
    {"type": "AWS::IAM::Role", "ARN": "arn:aws:iam::123456789012:role/gates-github-actions"}
  ],
  "managementEvent": true
}
```

拒否の記録には、JWT の `aud` と `sub` が `principalId` と `userName` に残り、引き受けようとした Role が `resources` に残ります。
`requestParameters` は `null` で、拒否の理由が Trust Policy のどの条件かは記録されません。

比べるため、同じ試験の時間帯に、immutable `sub` のリポジトリ（`example-repo`）のワークフローが `configure-aws-credentials` で成功させた引き受けの記録も確かめました。
記録の主な項目は次のとおりです。
送信元 IP アドレス、`userAgent`、各 ID は省き、`accessKeyId` は AWS の資料の例の値に、`sessionToken` はプレースホルダーに置き換えています。

```json
{
  "userIdentity": {
    "type": "WebIdentityUser",
    "principalId": "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com:sts.amazonaws.com:repo:octocat@583231/example-repo@1234567890:ref:refs/heads/example-branch",
    "userName": "repo:octocat@583231/example-repo@1234567890:ref:refs/heads/example-branch",
    "identityProvider": "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"
  },
  "eventSource": "sts.amazonaws.com",
  "eventName": "AssumeRoleWithWebIdentity",
  "requestParameters": {
    "webIdentityTokenId": "<id>",
    "roleArn": "arn:aws:iam::123456789012:role/gates-github-actions",
    "roleSessionName": "GitHubActions",
    "durationSeconds": 3600
  },
  "responseElements": {
    "credentials": {
      "accessKeyId": "ASIAIOSFODNN7EXAMPLE",
      "sessionToken": "<session token>",
      "expiration": "<1 時間後の時刻>"
    },
    "subjectFromWebIdentityToken": "repo:octocat@583231/example-repo@1234567890:ref:refs/heads/example-branch",
    "assumedRoleUser": {
      "assumedRoleId": "<role id>:GitHubActions",
      "arn": "arn:aws:sts::123456789012:assumed-role/gates-github-actions/GitHubActions"
    },
    "provider": "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com",
    "audience": "sts.amazonaws.com"
  },
  "resources": [
    {"type": "AWS::IAM::Role", "ARN": "arn:aws:iam::123456789012:role/gates-github-actions"}
  ],
  "managementEvent": true
}
```

成功の記録では、`sub` が `userName` と `responseElements.subjectFromWebIdentityToken` に、引き受けた Role とセッション名が `requestParameters` と `responseElements.assumedRoleUser` に残ります。
`responseElements.credentials` には、発行した一時認証情報の `accessKeyId`、`sessionToken`、`expiration` が残り、`secretAccessKey` はありませんでした。
IAM User Guide も、`AssumeRoleWithWebIdentity` の記録に `secretAccessKey` を除く応答の全体を含めると記述しています。
JWT そのものは、成功と拒否のどちらの記録にもありません。

## Workflow File

引き受けの試験に使用したワークフローファイルは次のとおりです。
作業ブランチへの push で起動しました。

```yaml
name: GATES Legacy Sub
on:
  push:
    branches:
      - example-branch
jobs:
  assume:
    runs-on: ubuntu-latest
    timeout-minutes: 5
    permissions:
      id-token: write
    env:
      AWS_REGION: ap-northeast-1
      ROLE_ARN: arn:aws:iam::${{ secrets.GATES_AWS_ACCOUNT_ID }}:role/gates-github-actions
    steps:
      - name: Assume role with web identity
        shell: bash
        run: |
          set -euo pipefail
          umask 077
          echo "::add-mask::${ACTIONS_ID_TOKEN_REQUEST_TOKEN}"
          dir="$(mktemp -d "${RUNNER_TEMP}/gates.XXXXXX")"
          trap 'rm -rf "${dir}"' EXIT
          curl --silent --show-error --fail \
            -H "Authorization: Bearer ${ACTIONS_ID_TOKEN_REQUEST_TOKEN}" \
            -H "Accept: application/json" \
            "${ACTIONS_ID_TOKEN_REQUEST_URL}&audience=sts.amazonaws.com" \
            | jq -j '.value' > "${dir}/id_token"
          echo "::add-mask::$(< "${dir}/id_token")"
          # sub と aud は資格情報ではない。JWT の値は出さない
          jq -R '
            def decode: gsub("-"; "+") | gsub("_"; "/") | . + ("=" * ((4 - length % 4) % 4)) | @base64d | fromjson;
            split(".")[1] | decode | {sub, aud}
          ' < "${dir}/id_token"
          status=0
          aws sts assume-role-with-web-identity \
            --role-arn "${ROLE_ARN}" \
            --role-session-name legacy-sub \
            --web-identity-token "file://${dir}/id_token" \
            --query 'AssumedRoleUser.Arn' --output text > "${dir}/result.txt" 2> "${dir}/stderr.txt" || status=$?
          echo "exit: ${status}"
          cat "${dir}/stderr.txt"
          if [[ "${status}" == 0 ]]; then
            echo "::error::the role was assumed with a legacy sub"
            exit 1
          fi
          grep -q 'AccessDenied' "${dir}/stderr.txt"
```

## References

- [GitHub — OpenID Connect reference](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)
- [AWS — AssumeRoleWithWebIdentity](https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRoleWithWebIdentity.html)
- [AWS — AWS CLI sts assume-role-with-web-identity](https://docs.aws.amazon.com/cli/latest/reference/sts/assume-role-with-web-identity.html)
- [AWS — Logging IAM and AWS STS API calls with AWS CloudTrail](https://docs.aws.amazon.com/IAM/latest/UserGuide/cloudtrail-integration.html)
