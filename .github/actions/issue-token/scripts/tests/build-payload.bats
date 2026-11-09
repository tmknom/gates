#!/usr/bin/env bats
# build-payload.sh の試験
# shellcheck disable=SC2030,SC2031 # 各試験は別のサブシェルで動き、環境変数の変更をその試験に閉じる

setup() {
  load test_helper
  setup_action_test

  SCRIPT="${SCRIPTS_DIR}/build-payload.sh"
  JWT='eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJyZXBvIn0.c2lnbmF0dXJl'
  export ID_TOKEN_FILE="${WORK}/id-token"
  export OUT="${WORK}/payload.json"
  export REPOSITORIES='example-org/repo-a'
  export PERMISSIONS='contents:read'
  printf '%s' "${JWT}" >"${ID_TOKEN_FILE}"
}

# どの事例でも、JWT を出力と jq の引数に出さず、JWT のファイルを残さない
teardown() {
  assert_not_exposed "${JWT}"
  [[ ! -e "${ID_TOKEN_FILE}" ]]
}

# OUT の JSON が期待する JSON と等しいことを確かめる
assert_payload() {
  # shellcheck disable=SC2016 # jq の変数を展開させない
  "${REAL_JQ}" -e --argjson expected "$1" '. == $expected' "${OUT}" >/dev/null
}

# 失敗し、Payload を書かず、すべての注釈を標準出力に出したことを確かめる
assert_rejected() {
  [[ "${status}" -eq 1 ]]
  [[ ! -e "${OUT}" ]]
  [[ "${output}" == "::error::"* ]]
  [[ -z "${stderr}" ]]
}

@test "対象 1 件の Payload を作る" {
  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  assert_payload "{\"version\":1,\"id_token\":\"${JWT}\",\"repositories\":[\"example-org/repo-a\"],\"permissions\":{\"contents\":\"read\"}}"
}

@test "対象 2 件を入力の順の配列にする" {
  export REPOSITORIES='example-org/repo-b,example-org/repo-a'

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  "${REAL_JQ}" -e '.repositories == ["example-org/repo-b", "example-org/repo-a"]' "${OUT}"
}

@test "要素の前後の空白を除く" {
  export REPOSITORIES=' example-org/repo-a , example-org/repo-b '
  export PERMISSIONS=$'\tcontents:read '

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  "${REAL_JQ}" -e '.repositories == ["example-org/repo-a", "example-org/repo-b"]' "${OUT}"
  "${REAL_JQ}" -e '.permissions == {"contents": "read"}' "${OUT}"
}

@test "空の要素があれば、その位置を示して失敗する" {
  export REPOSITORIES='example-org/repo-a,,example-org/repo-b'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *"repositories"*"element 2 is empty"* ]]
}

@test "末尾の , は空の要素として失敗する" {
  export REPOSITORIES='example-org/repo-a,'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *"repositories"*"element 2 is empty"* ]]
}

@test "空白だけの要素は空の要素として失敗する" {
  export PERMISSIONS='contents:read, '

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *"permissions"*"element 2 is empty"* ]]
}

@test "空の入力は、1 番目の要素が空であるとして失敗する" {
  export REPOSITORIES=''

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *"repositories"*"element 1 is empty"* ]]
}

@test "大文字小文字だけが違う対象を重複として、両方の要素を示して失敗する" {
  export REPOSITORIES='example-org/repo-a,Example-Org/Repo-A'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *'"example-org/repo-a"'* ]]
  [[ "${output}" == *'"Example-Org/Repo-A"'* ]]
  [[ "${output}" == *"duplicate"* ]]
}

@test "/ を含まない対象は失敗する" {
  export REPOSITORIES='example-org'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *'"example-org"'*"owner/repo"* ]]
}

@test "/ を 2 つ含む対象は失敗する" {
  export REPOSITORIES='a/b/c'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *'"a/b/c"'*"owner/repo"* ]]
}

@test "/ の前が空の対象は失敗する" {
  export REPOSITORIES='/repo-a'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *'"/repo-a"'* ]]
}

@test "/ の後が空の対象は失敗する" {
  export REPOSITORIES='example-org/'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *'"example-org/"'* ]]
}

@test "権限 2 つを 1 つのオブジェクトにする" {
  export PERMISSIONS='contents:read,metadata:read'

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  "${REAL_JQ}" -e '.permissions == {"contents": "read", "metadata": "read"}' "${OUT}"
}

@test "権限名が重複すると、両方の要素を示して失敗する" {
  export PERMISSIONS='contents:read,contents:write'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *'"contents:read"'* ]]
  [[ "${output}" == *'"contents:write"'* ]]
  [[ "${output}" == *"duplicate"* ]]
}

@test "大文字小文字だけが違う権限名は重複にしない" {
  export PERMISSIONS='contents:read,Contents:read'

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  "${REAL_JQ}" -e '.permissions == {"contents": "read", "Contents": "read"}' "${OUT}"
}

@test ": を含まない権限は失敗する" {
  export PERMISSIONS='contents'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *'"contents"'*"name:level"* ]]
}

@test ": の後が空の権限は失敗する" {
  export PERMISSIONS='contents:'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *'"contents:"'* ]]
}

@test ": の前が空の権限は失敗する" {
  export PERMISSIONS=':read'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *'":read"'* ]]
}

@test ": を 2 つ含む権限は失敗する" {
  export PERMISSIONS='a:b:c'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *'"a:b:c"'* ]]
}

@test "満たさない条件をすべて、1 行に 1 つずつ示す" {
  export REPOSITORIES='example-org,,a/b/c'
  export PERMISSIONS='contents'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "$(grep -c '^::error::' <<<"${output}")" -eq 4 ]]
}

@test "注釈の中の要素の改行と % を符号化する" {
  export REPOSITORIES=$'example-org\n::add-mask::injected,100%'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  refute grep -q '^::add-mask::' <<<"${output}"
  [[ "${output}" == *'"100%25"'* ]]
}

@test "区切りの前後に空白がある要素は、書き換えずに失敗する" {
  export REPOSITORIES='example-org /repo-a'
  export PERMISSIONS='contents:read,contents :write'

  run --separate-stderr "${SCRIPT}"

  assert_rejected
  [[ "${output}" == *'"example-org /repo-a"'*"owner/repo"* ]]
  [[ "${output}" == *'"contents :write"'*"name:level"* ]]
}

@test "JWT のファイルを消せなければ、Payload を書けても失敗し、Payload を残さない" {
  export RM_STUB_FAIL="${ID_TOKEN_FILE}"

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ ! -e "${OUT}" ]]
  [[ "${output}" == *"::error::"*"ID_TOKEN_FILE"* ]]
  "${REAL_RM}" -- "${ID_TOKEN_FILE}"
}

@test "前からある OUT を消せなければ、そのことを 1 度だけ出して失敗する" {
  printf 'stale' >"${OUT}"
  export RM_STUB_FAIL="${OUT}"

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "$(grep -c '^::error::' <<<"${output}")" -eq 1 ]]
  [[ "${output}" == *"::error::"*"OUT"* ]]
  "${REAL_RM}" -- "${OUT}"
}

@test "名前と権限の文字種は検証しない" {
  export REPOSITORIES='example_org/repo.a'
  export PERMISSIONS='contents:admin'

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  assert_payload "{\"version\":1,\"id_token\":\"${JWT}\",\"repositories\":[\"example_org/repo.a\"],\"permissions\":{\"contents\":\"admin\"}}"
}

@test "JWT の \" と \\ を JSON の文字列として正しく取り込む" {
  JWT='header"quote\back\\slash'
  printf '%s' "${JWT}" >"${ID_TOKEN_FILE}"

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  [[ "$("${REAL_JQ}" -j '.id_token' "${OUT}")" == "${JWT}" ]]
}

@test "JWT は jq の引数ではなく、--rawfile でファイルから読む" {
  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  grep -qxF -- '--rawfile' "${STUB_LOG_DIR}/jq.args"
  grep -qxF -- "${ID_TOKEN_FILE}" "${STUB_LOG_DIR}/jq.args"
}

@test "JWT のファイルがなければ、段階だけを示して失敗する" {
  rm "${ID_TOKEN_FILE}"

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ ! -e "${OUT}" ]]
  [[ "${output}" == "::error::"* ]]
  [[ "${#lines[@]}" -eq 1 && -z "${stderr}" ]]
}

@test "環境変数が欠けていれば、その名前を示して失敗する" {
  unset PERMISSIONS

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ ! -e "${OUT}" ]]
  [[ "${output}" == "::error::"*"PERMISSIONS"* ]]
}

@test "version を整数の 1 として書く" {
  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  grep -q '"version":1[,}]' "${OUT}"
}

@test "OUT が他者も読める権限ですでにあっても、所有者だけが読み書きできる OUT を書く" {
  printf 'stale' >"${OUT}"
  chmod 644 "${OUT}"

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  assert_owner_only "${OUT}"
}
