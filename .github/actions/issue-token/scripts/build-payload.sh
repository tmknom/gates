#!/usr/bin/env bash
# 入力のリストの構造を検証し、Lambda の Invoke Payload を OUT に書く。
# 名前と権限の文字種、対象の件数、許可条件は検証せず、Lambda に任せる
#
# 環境変数
#   REPOSITORIES   カンマ区切りの owner/repo
#   PERMISSIONS    カンマ区切りの permission:level
#   ID_TOKEN_FILE  GATES 用 OIDC JWT を置いたファイルのパス。成否によらず削除する
#   OUT            Payload を書くファイルのパス
set -euo pipefail
umask 077

readonly STAGE='Failed to build the payload'

# JWT のファイルは成否によらず消し、消せなければ失敗にする。
# 失敗やシグナルで終えたときは、JWT を含み得る書きかけの Payload も消す。
# 削除の失敗で後の削除を飛ばさないよう、errexit を外して最後まで進め、終了コードを明示して返す
cleanup() {
  local status=$?
  set +e
  if [[ -n "${ID_TOKEN_FILE:-}" ]] && ! rm -f -- "${ID_TOKEN_FILE}"; then
    echo "::error::${STAGE}: could not remove the file at ID_TOKEN_FILE"
    status=1
  fi
  if [[ "${status}" -ne 0 && -n "${OUT:-}" ]] && ! rm -f -- "${OUT}"; then
    echo "::error::${STAGE}: could not remove the file at OUT"
  fi
  exit "${status}"
}
trap cleanup EXIT

# REPOSITORIES と PERMISSIONS は空でもよく、空の要素として検証する
for name in REPOSITORIES PERMISSIONS ID_TOKEN_FILE OUT; do
  if [[ ! -v "${name}" || (-z "${!name}" && "${name}" != REPOSITORIES && "${name}" != PERMISSIONS) ]]; then
    echo "::error::${STAGE}: ${name} is not set"
    exit 1
  fi
done

# 前からある OUT を消してから書く。既存のファイルに書くと元の権限が残るため。
# 消せなければ、終了時の cleanup がもう一度消そうとして注釈を出すため、ここでは出さない
rm -f -- "${OUT}" || exit 1

# 検証の条件を 1 か所に集めるため、分割、検証、変換を 1 つのフィルタで行う。
# 満たさない条件があれば、注釈の行を halt_error で返す。終了コードは jq 自身の失敗（2、3、5）と分ける
readonly INVALID_INPUT=10
# shellcheck disable=SC2016 # jq の変数を展開させない
readonly FILTER='
  # カンマで分け、要素の前後の空白を除く。split は空文字列を空の配列にするため、空の入力は空の要素 1 つとして扱う
  def elements:
    (if . == "" then [""] else split(",") end)
    | map(sub("^[ \t\r\n]+"; "") | sub("[ \t\r\n]+$"; ""));

  # 注釈に要素を示す。JSON の文字列にして改行を除き、注釈のデータで特別な意味を持つ % を符号化する
  def show: tojson | gsub("%"; "%25");

  # 区切りでちょうど 2 つに分かれ、どちらも空でなく、空白で始まりも終わりもしない。
  # 区切りの前後の空白は除かずに拒否する。除くと、利用者が書いた要素と送る要素が食い違うため
  def is_pair($separator):
    split($separator) | length == 2 and all(.[]; . != "" and (test("^[ \t\r\n]|[ \t\r\n]$") | not));

  def list_errors($name; $items; $separator; $form; key; $compared):
    [$items | to_entries[] | select(.value == "") | "\($name): element \(.key + 1) is empty"]
    + [$items[] | select(. != "" and (is_pair($separator) | not))
        | "\($name): \(show) is not in the form \($form)"]
    + [$items | map(select(is_pair($separator))) | group_by(key)[] | select(length > 1)
        | "\($name): \(map(show) | join(", ")) are duplicates \($compared)"];

  ($repositories | elements) as $repos
  | ($permissions | elements) as $perms
  | list_errors("repositories"; $repos; "/"; "owner/repo"; ascii_downcase; "(compared case-insensitively)")
    + list_errors("permissions"; $perms; ":"; "name:level"; split(":")[0]; "(same permission name)")
  | if length > 0 then
      map("::error::Invalid input: \(.)\n") | add | halt_error($invalid_input)
    else
      {
        version: 1,
        id_token: $id_token,
        repositories: $repos,
        permissions: ($perms | map(split(":") | {(.[0]): .[1]}) | add)
      }
    end
'

# JWT はコマンドの引数に置かず、--rawfile でファイルから読む。
# 標準出力は OUT に、halt_error の注釈は標準エラーから変数に受ける
jq_status=0
errors="$(jq --null-input --compact-output \
  --arg repositories "${REPOSITORIES}" \
  --arg permissions "${PERMISSIONS}" \
  --argjson invalid_input "${INVALID_INPUT}" \
  --rawfile id_token "${ID_TOKEN_FILE}" \
  "${FILTER}" 2>&1 >"${OUT}")" || jq_status=$?

if [[ "${jq_status}" -ne 0 ]]; then
  # ワークフローコマンドは標準出力から読まれるため、注釈を標準出力に出す。
  # jq 自身の失敗の文言は値の一部を含み得るため出さない
  if [[ "${jq_status}" -eq "${INVALID_INPUT}" ]]; then
    printf '%s\n' "${errors}"
  else
    echo "::error::${STAGE}: jq exited with ${jq_status}"
  fi
  exit 1
fi
