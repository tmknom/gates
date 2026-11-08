#!/usr/bin/env bats
# request-id-token.sh の試験
# shellcheck disable=SC2030,SC2031 # 各試験は別のサブシェルで動き、環境変数の変更をその試験に閉じる

setup() {
  load test_helper
  setup_action_test

  SCRIPT="${SCRIPTS_DIR}/request-id-token.sh"
  REQUEST_URL='https://token.actions.example/token?api-version=2.0'
  REQUEST_TOKEN='request-token-value'
  JWT='eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJyZXBvIn0.c2lnbmF0dXJl'
  export ACTIONS_ID_TOKEN_REQUEST_URL="${REQUEST_URL}"
  export ACTIONS_ID_TOKEN_REQUEST_TOKEN="${REQUEST_TOKEN}"
  export AUDIENCE='https://gates.example/audience'
  export OUT="${WORK}/id-token"
  export CURL_STUB_BODY="{\"count\":1,\"value\":\"${JWT}\"}"
}

@test "200 で value が返ると、OUT に JWT だけを書き、応答のファイルを残さない" {
  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  printf '%s' "${JWT}" | cmp - "${OUT}"
  [[ "$(ls -A "${WORK}")" == id-token ]]
}

@test "要求用トークンと JWT の 2 つをマスクし、要求用トークンを先にマスクする" {
  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  mapfile -t masks < <(grep '^::add-mask::' <<<"${output}")
  [[ "${#masks[@]}" -eq 2 ]]
  [[ "${masks[0]}" == "::add-mask::${REQUEST_TOKEN}" ]]
  [[ "${masks[1]}" == "::add-mask::${JWT}" ]]
  [[ "${lines[0]}" == "::add-mask::${REQUEST_TOKEN}" ]]
}

@test "URI エンコードした Audience を付けた URL に、JSON を受け付けて要求する" {
  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  grep -qxF "${REQUEST_URL}&audience=https%3A%2F%2Fgates.example%2Faudience" "${STUB_LOG_DIR}/curl.args"
  grep -qxF 'Accept: application/json' "${STUB_LOG_DIR}/curl.args"
}

@test "要求用トークンを含む Authorization ヘッダーを、引数ではなく標準入力から渡す" {
  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  grep -qxF -- '@-' "${STUB_LOG_DIR}/curl.args"
  [[ "$(cat "${STUB_LOG_DIR}/curl.stdin")" == "Authorization: Bearer ${REQUEST_TOKEN}" ]]
  refute grep -qF -- "${REQUEST_TOKEN}" "${STUB_LOG_DIR}/curl.args"
}

@test "400 が返ると、ステータスと本文を出して失敗し、OUT を書かない" {
  export CURL_STUB_STATUS=400
  export CURL_STUB_BODY='{"message":"Can not issue ID_TOKEN for audience"}'

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "${output}" == *"::error::"*"400"* ]]
  [[ "${output}" == *'Can not issue ID_TOKEN for audience'* ]]
  [[ ! -e "${OUT}" ]]
  [[ -z "$(ls -A "${WORK}")" ]]
}

@test "失敗の本文の改行と % を符号化し、1 つの注釈に収める" {
  export CURL_STUB_STATUS=400
  export CURL_STUB_BODY=$'line 1 100%\n::add-mask::injected'

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "${output}" == *'line 1 100%25%0A::add-mask::injected'* ]]
  refute grep -q '^::add-mask::injected' <<<"${output}"
}

@test "200 以外の 2xx が返ると、本文を出さずに失敗する" {
  export CURL_STUB_STATUS=203
  export CURL_STUB_BODY="{\"value\":\"${JWT}\"}"

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "${output}" == *"::error::"*"203"* ]]
  [[ "${output}" != *"${JWT}"* ]]
  [[ ! -e "${OUT}" ]]
}

@test "3xx が返ると、本文を出さずに失敗する" {
  export CURL_STUB_STATUS=302
  export CURL_STUB_BODY="{\"value\":\"${JWT}\"}"

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "${output}" == *"::error::"*"302"* ]]
  [[ "${output}" != *"${JWT}"* ]]
  [[ ! -e "${OUT}" ]]
}

@test "マスクする値の % を符号化し、Runner が戻した値をマスクさせる" {
  export ACTIONS_ID_TOKEN_REQUEST_TOKEN='request%0Atoken'
  export CURL_STUB_BODY='{"value":"jwt%25value"}'

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  [[ "${lines[0]}" == '::add-mask::request%250Atoken' ]]
  grep -qxF '::add-mask::jwt%2525value' <<<"${output}"
  printf '%s' 'jwt%25value' | cmp - "${OUT}"
}

@test "失敗の本文は先頭の 4096 バイトまで出す" {
  local head
  printf -v head '%4096s' ''
  head="${head// /a}"
  export CURL_STUB_STATUS=500
  export CURL_STUB_BODY="${head}tail-not-shown"

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "${output}" == *"${head}"* ]]
  [[ "${output}" != *'tail-not-shown'* ]]
}

@test "応答のファイルを消せなければ、JWT を取得できても失敗し、OUT を残さない" {
  export RM_STUB_FAIL="${WORK}/id-token-response.*"

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ ! -e "${OUT}" ]]
  [[ "${output}" == *"::error::"*"response"* ]]
}

@test "200 の応答に value がなければ、そのことを出して失敗し、本文を出さない" {
  export CURL_STUB_BODY='{"count":0,"other":"not-a-token"}'

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "${output}" == *"::error::"*"value"* ]]
  [[ "${output}" != *'not-a-token'* ]]
  [[ ! -e "${OUT}" ]]
}

@test "value が空文字列なら失敗する" {
  export CURL_STUB_BODY='{"value":""}'

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ ! -e "${OUT}" ]]
}

@test "value が改行を含めば、マスクせずに失敗し、どの行も出さない" {
  export CURL_STUB_BODY='{"value":"secretA\nsecretB"}'

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "${output}" != *secretA* && "${output}" != *secretB* ]]
  [[ ! -e "${OUT}" ]]
}

@test "応答に JSON が 2 つあれば失敗し、どちらの値も出さない" {
  export CURL_STUB_BODY='{"value":"secretA"}{"value":"secretB"}'

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "${output}" != *secretA* && "${output}" != *secretB* ]]
  [[ ! -e "${OUT}" ]]
}

@test "curl に設定ファイルを読ませず、https だけを許す" {
  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  [[ "$(head -n 1 "${STUB_LOG_DIR}/curl.args")" == --disable ]]
  grep -qxF -- '=https' "${STUB_LOG_DIR}/curl.args"
}

@test "ACTIONS_ID_TOKEN_REQUEST_TOKEN がなければ、id-token: write を促して失敗し、curl を呼ばない" {
  unset ACTIONS_ID_TOKEN_REQUEST_TOKEN

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "${output}" == *"::error::"*"id-token: write"* ]]
  [[ ! -e "${STUB_LOG_DIR}/curl.args" ]]
}

@test "AUDIENCE がなければ、段階を示して失敗し、curl を呼ばない" {
  unset AUDIENCE

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "${output}" == *"::error::"*"AUDIENCE"* ]]
  [[ ! -e "${STUB_LOG_DIR}/curl.args" ]]
}

@test "ACTIONS_ID_TOKEN_REQUEST_URL がなければ、id-token: write を促して失敗し、curl を呼ばない" {
  unset ACTIONS_ID_TOKEN_REQUEST_URL

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "${output}" == *"::error::"*"id-token: write"* ]]
  [[ ! -e "${STUB_LOG_DIR}/curl.args" ]]
  [[ ! -e "${OUT}" ]]
}

@test "curl が失敗すると、終了コードを出して失敗し、OUT を書かない" {
  export CURL_STUB_EXIT=7

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "${output}" == *"::error::"*"7"* ]]
  [[ ! -e "${OUT}" ]]
  [[ -z "$(ls -A "${WORK}")" ]]
}

@test "JWT と要求用トークンは、マスクの行のほかに出力にも引数にも現れない" {
  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  assert_not_exposed "${JWT}" "::add-mask::${JWT}"
  assert_not_exposed "${REQUEST_TOKEN}" "::add-mask::${REQUEST_TOKEN}"
}

@test "OUT が他者も読める権限ですでにあっても、所有者だけが読み書きできる OUT を書く" {
  printf 'stale' >"${OUT}"
  chmod 644 "${OUT}"

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  printf '%s' "${JWT}" | cmp - "${OUT}"
  assert_owner_only "${OUT}"
}

@test "要求用トークンがなければ、前からあった OUT も消す" {
  printf 'stale-jwt' >"${OUT}"
  unset ACTIONS_ID_TOKEN_REQUEST_TOKEN

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ ! -e "${OUT}" ]]
}

@test "OUT がなければ、そのことだけを出して失敗する" {
  unset OUT

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "$(grep -c '^::error::' <<<"${output}")" -eq 1 ]]
  [[ "${output}" == *"::error::"*"OUT is not set"* ]]
  [[ -z "${stderr}" ]]
}

@test "前からある OUT を消せなければ、そのことを 1 度だけ出して失敗する" {
  printf 'stale-jwt' >"${OUT}"
  export RM_STUB_FAIL="${OUT}"

  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 1 ]]
  [[ "$(grep -c '^::error::' <<<"${output}")" -eq 1 ]]
  [[ "${output}" == *"::error::"*"OUT"* ]]
  [[ ! -e "${STUB_LOG_DIR}/curl.args" ]]
}

@test "応答を待ち続けないよう、curl に接続と全体のタイムアウトを渡し、再試行させない" {
  run --separate-stderr "${SCRIPT}"

  [[ "${status}" -eq 0 ]]
  grep -qxF -- '--connect-timeout' "${STUB_LOG_DIR}/curl.args"
  grep -qxF -- '--max-time' "${STUB_LOG_DIR}/curl.args"
  refute grep -qE -- '^--retry' "${STUB_LOG_DIR}/curl.args"
}
