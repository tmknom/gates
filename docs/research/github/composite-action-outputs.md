# GitHub Composite Action Outputs

本ドキュメントでは、リポジトリ内の Composite Action が内部ステップでマスクした値を、Output として呼び出し元に渡したときの挙動を記述します。
調べたのは、マスクが呼び出し元の後続ステップでも効くか、値が欠けずに届くか、値をジョブの Output として次のジョブへ渡せるかです。
GitHub Actions の実環境で実行し、ログへ出力した値を掲載しています。

- 調査日: **2026/11/09**
- 検証環境: 個人アカウントのプライベートリポジトリ。`runs-on: ubuntu-latest`
- 起動: 作業ブランチへの push

オーナー名は `octocat`、リポジトリ名は `example-repo`、ブランチ名は `example-branch` に置き換えています。
調査に使った値は、ワークフローの中で `openssl rand` で作った乱数の文字列だけで、資格情報ではありません。

## Probe

調査用の Composite Action を `.github/actions/probe/` に置き、ワークフローから `uses: ./.github/actions/probe` で呼びました。

Action の内部ステップは、下の表の値を作って `::add-mask::` でマスクし、それぞれを `GITHUB_OUTPUT` の 1 行の形式と区切り文字の形式で書きます。
値の SHA-256 は、マスクせずに別の Output に書きます。
呼び出し元のワークフローは `::add-mask::` を実行しません。

| 値 | 作り方 | 長さ |
|---|---|---|
| base64 の値 | `openssl rand -base64 46` を、`+` と `/` の両方を含むまで繰り返す | 64 文字。末尾は `==` |
| 長い値 | `openssl rand -base64 375` の出力から改行を除く | 500 文字 |

1 行の形式で値の `=` が欠けないかを確かめるため、3 の倍数でない 46 バイトにして、base64 の値の末尾に `=` が付くようにしました。

| 形式 | `GITHUB_OUTPUT` に書いた内容 |
|---|---|
| 1 行の形式 | `name=value` |
| 区切り文字の形式 | `name<<EOF_<乱数の 32 桁の 16 進数>`、値、区切り文字の 3 行 |

## Mask

Action の内部ステップでマスクした値は、呼び出し元のジョブの後続ステップでもマスクされました。
呼び出し元のステップは、Action の Output を `steps.probe.outputs.<name>` から受け取ってログに表示しました。

| ログに表示した箇所 | 渡し方 | ログ |
|---|---|---|
| Action の内部ステップの `echo` | `echo` | `echo inside the action: ***` |
| 呼び出し元のステップの開始時に表示される `env:` | `env:` に `${{ steps.probe.outputs.<name> }}` | マスクした値の Output はすべて `***` |
| 呼び出し元のステップの `echo` | `env:` で受け取った環境変数を `echo` | マスクした値の Output はすべて `***` |
| 呼び出し元のステップの開始時に表示される `run:` のスクリプト | `run:` の中に `${{ steps.probe.outputs.b64-single }}` を書く | `Run echo "inline b64-single: ***"` |
| 呼び出し元のステップの `echo` | 同上 | `inline b64-single: ***` |

base64 の値の先頭 8 文字を `${B64_SINGLE:0:8}` で取り出して `echo` すると、その 8 文字はマスクされずにログに表示されました。

## Output

呼び出し元が受け取った値の SHA-256 は、マスクした値の Output のすべてで、Action の内部で計算した値と一致しました。

```text
b64-single: match (length 64)
b64-heredoc: match (length 64)
long-single: match (length 500)
long-heredoc: match (length 500)
```

1 行の形式で、`=` を値に含む base64 の値も欠けずに届きました。

## Job Outputs

呼び出し元のジョブの `outputs` に、マスクした値の Output とマスクしていない SHA-256 の Output を置き、`needs` で次のジョブに渡しました。
ジョブの完了時に次のログが出ました。

```text
##[warning]Skip output 'b64-single' since it may contain secret.
Set output 'b64-sha256'
```

次のジョブでは、マスクした値の Output は空文字列（長さ 0）で、SHA-256 の Output は 64 文字で届きました。

GitHub の Workflow commands の資料は、`add-mask` について「after you mask a value, you won't be able to set that value as an output」と記述し、ジョブやワークフローをまたいでマスクした値を渡すにはシークレットストアを使うよう案内しています。
観測では、マスクした値も内部ステップの Output に設定でき、Composite Action の Output として同じジョブの後続ステップに届きました。
ジョブの Output には設定されませんでした。

## Action Files

`.github/actions/probe/action.yml`

```yaml
name: Probe
description: Probe how a composite action passes masked values to the caller through outputs
outputs:
  b64-single:
    description: Random base64 value written as name=value
    value: ${{ steps.generate.outputs.b64-single }}
  b64-heredoc:
    description: Random base64 value written with a delimiter
    value: ${{ steps.generate.outputs.b64-heredoc }}
  b64-sha256:
    description: SHA-256 of the base64 value, not masked
    value: ${{ steps.generate.outputs.b64-sha256 }}
  long-single:
    description: Random 500 character value written as name=value
    value: ${{ steps.generate.outputs.long-single }}
  long-heredoc:
    description: Random 500 character value written with a delimiter
    value: ${{ steps.generate.outputs.long-heredoc }}
  long-sha256:
    description: SHA-256 of the 500 character value, not masked
    value: ${{ steps.generate.outputs.long-sha256 }}
runs:
  using: composite
  steps:
    - name: Generate and mask values
      id: generate
      shell: bash
      run: bash "${GITHUB_ACTION_PATH}/generate.sh"
```

`.github/actions/probe/generate.sh`

```bash
#!/usr/bin/env bash
# Generates random values, masks them, and writes them to GITHUB_OUTPUT in two forms.
# The values are random strings only. They are not credentials.
set -euo pipefail

# 46 bytes encode to 64 characters ending with "==". Retry until "+" and "/" also appear.
b64=""
while [[ "${b64}" != *+* || "${b64}" != */* ]]; do
  b64="$(openssl rand -base64 46)"
done
# 375 bytes encode to 500 characters. tr joins the wrapped lines into one.
long="$(openssl rand -base64 375 | tr -d '\n')"

echo "::add-mask::${b64}"
echo "::add-mask::${long}"

delimiter="EOF_$(openssl rand -hex 16)"
{
  echo "b64-single=${b64}"
  echo "b64-heredoc<<${delimiter}"
  echo "${b64}"
  echo "${delimiter}"
  echo "b64-sha256=$(printf '%s' "${b64}" | sha256sum | cut -d' ' -f1)"
  echo "long-single=${long}"
  echo "long-heredoc<<${delimiter}"
  echo "${long}"
  echo "${delimiter}"
  echo "long-sha256=$(printf '%s' "${long}" | sha256sum | cut -d' ' -f1)"
} >> "${GITHUB_OUTPUT}"

echo "b64 length: ${#b64}, padding: ${b64: -2}"
echo "long length: ${#long}"
echo "echo inside the action: ${b64}"
```

## Workflow File

```yaml
name: Composite Action Outputs
on:
  workflow_dispatch:
  push:
    branches:
      - example-branch

permissions: {}

jobs:
  probe:
    runs-on: ubuntu-latest
    timeout-minutes: 5
    permissions:
      contents: read
    outputs:
      b64-single: ${{ steps.probe.outputs.b64-single }}
      b64-sha256: ${{ steps.probe.outputs.b64-sha256 }}
    steps:
      - name: Checkout
        uses: actions/checkout@d23441a48e516b6c34aea4fa41551a30e30af803 # v6.1.0
        with:
          persist-credentials: false

      - name: Probe
        id: probe
        uses: ./.github/actions/probe

      # 値は乱数の文字列で、資格情報ではない
      - name: Echo outputs through env
        shell: bash
        env:
          B64_SINGLE: ${{ steps.probe.outputs.b64-single }}
          B64_HEREDOC: ${{ steps.probe.outputs.b64-heredoc }}
          LONG_SINGLE: ${{ steps.probe.outputs.long-single }}
          LONG_HEREDOC: ${{ steps.probe.outputs.long-heredoc }}
        run: |
          echo "b64-single: ${B64_SINGLE}"
          echo "b64-heredoc: ${B64_HEREDOC}"
          echo "long-single: ${LONG_SINGLE}"
          echo "long-heredoc: ${LONG_HEREDOC}"
          echo "b64 prefix (8 chars): ${B64_SINGLE:0:8}"

      - name: Echo an output through an inline expression
        shell: bash
        run: |
          echo "inline b64-single: ${{ steps.probe.outputs.b64-single }}"

      - name: Compare hashes
        shell: bash
        env:
          B64_SINGLE: ${{ steps.probe.outputs.b64-single }}
          B64_HEREDOC: ${{ steps.probe.outputs.b64-heredoc }}
          B64_SHA256: ${{ steps.probe.outputs.b64-sha256 }}
          LONG_SINGLE: ${{ steps.probe.outputs.long-single }}
          LONG_HEREDOC: ${{ steps.probe.outputs.long-heredoc }}
          LONG_SHA256: ${{ steps.probe.outputs.long-sha256 }}
        run: |
          set -euo pipefail
          check() {
            local name="$1" value="$2" expected="$3" actual
            actual="$(printf '%s' "${value}" | sha256sum | cut -d' ' -f1)"
            if [[ "${actual}" == "${expected}" ]]; then
              echo "${name}: match (length ${#value})"
            else
              echo "${name}: MISMATCH (length ${#value})"
            fi
          }
          check b64-single "${B64_SINGLE}" "${B64_SHA256}"
          check b64-heredoc "${B64_HEREDOC}" "${B64_SHA256}"
          check long-single "${LONG_SINGLE}" "${LONG_SHA256}"
          check long-heredoc "${LONG_HEREDOC}" "${LONG_SHA256}"

  next-job:
    needs: probe
    runs-on: ubuntu-latest
    timeout-minutes: 5
    permissions: {}
    steps:
      - name: Read job outputs
        shell: bash
        env:
          B64_SINGLE: ${{ needs.probe.outputs.b64-single }}
          B64_SHA256: ${{ needs.probe.outputs.b64-sha256 }}
        run: |
          echo "b64-single length in next job: ${#B64_SINGLE}"
          echo "b64-sha256 length in next job: ${#B64_SHA256}"
```

## References

- [GitHub — Workflow commands for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)
- [GitHub — Metadata syntax reference](https://docs.github.com/en/actions/reference/workflows-and-actions/metadata-syntax)
