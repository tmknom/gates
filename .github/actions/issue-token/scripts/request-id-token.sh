#!/usr/bin/env bash
# GATES 用 OIDC JWT を取得し、末尾の改行なしで OUT に書く
#
# 環境変数
#   AUDIENCE  GATES 用 OIDC JWT の Audience
#   OUT       JWT を書くファイルのパス。作業ディレクトリの中に置き、応答のファイルも同じディレクトリに作る
#   ACTIONS_ID_TOKEN_REQUEST_URL、ACTIONS_ID_TOKEN_REQUEST_TOKEN  Runner が設定する
set -euo pipefail
umask 077

# 注釈とマスクのデータを 1 行に収める。Runner はデータの %25、%0D、%0A を戻してから使う。
# 改行が残ると、続く行をワークフローコマンドとして読ませ得る。% が残ると、戻した後の別の値をマスクしてしまう
escape_data() {
  local data="$1"
  data="${data//'%'/%25}"
  data="${data//$'\r'/%0D}"
  data="${data//$'\n'/%0A}"
  printf '%s' "${data}"
}

# マスクより前に実行したコマンドの出力にはマスクが効かないため、ほかのコマンドより先にマスクする
if [[ -n "${ACTIONS_ID_TOKEN_REQUEST_TOKEN:-}" ]]; then
  printf '::add-mask::%s\n' "$(escape_data "${ACTIONS_ID_TOKEN_REQUEST_TOKEN}")"
fi

readonly STAGE='Failed to get the GATES OIDC JWT'

# JWT を含み得る応答のファイルは成否によらず消し、消せなければ失敗にする。
# 失敗やシグナルで終えたときは、JWT を含み得る OUT も消す。前提を確かめる前に設定し、前提を欠いた失敗でも OUT を残さない。
# 削除の失敗で後の削除を飛ばさないよう、errexit を外して最後まで進め、終了コードを明示して返す
response=''
cleanup() {
  local status=$?
  set +e
  if [[ -n "${response}" ]] && ! rm -f -- "${response}"; then
    echo "::error::${STAGE}: could not remove the response file"
    status=1
  fi
  if [[ "${status}" -ne 0 && -n "${OUT:-}" ]] && ! rm -f -- "${OUT}"; then
    echo "::error::${STAGE}: could not remove the file at OUT"
  fi
  exit "${status}"
}
trap cleanup EXIT

if [[ -z "${ACTIONS_ID_TOKEN_REQUEST_URL:-}" || -z "${ACTIONS_ID_TOKEN_REQUEST_TOKEN:-}" ]]; then
  echo "::error::${STAGE}: ACTIONS_ID_TOKEN_REQUEST_URL or ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set. Declare 'permissions: id-token: write' for the job"
  exit 1
fi

for name in AUDIENCE OUT; do
  if [[ -z "${!name:-}" ]]; then
    echo "::error::${STAGE}: ${name} is not set"
    exit 1
  fi
done

# 前からある OUT を消してから書く。既存のファイルに書くと元の権限が残るため。
# 消せなければ、終了時の cleanup がもう一度消そうとして注釈を出すため、ここでは出さない
rm -f -- "${OUT}" || exit 1

if ! response="$(mktemp "$(dirname "${OUT}")/id-token-response.XXXXXX")"; then
  echo "::error::${STAGE}: could not create a file for the response"
  exit 1
fi

encoded_audience="$(jq --null-input --raw-output --arg audience "${AUDIENCE}" '$audience | @uri')"

# コマンドの引数は同じ Runner のほかのプロセスから読めるため、要求用トークンを含むヘッダーは標準入力から渡す。
# --disable は最初の引数でだけ効き、~/.curlrc の設定（ヘッダーを記録する --trace など）を読ませない。
# 応答が届かないときにジョブのタイムアウトまで待たないよう、接続と全体の時間を区切る
curl_status=0
http_status="$(printf 'Authorization: Bearer %s\n' "${ACTIONS_ID_TOKEN_REQUEST_TOKEN}" \
  | curl --disable --proto =https --silent --show-error \
    --connect-timeout 10 --max-time 30 \
    --header @- --header 'Accept: application/json' \
    --output "${response}" --write-out '%{http_code}' \
    "${ACTIONS_ID_TOKEN_REQUEST_URL}&audience=${encoded_audience}")" || curl_status=$?

if [[ "${curl_status}" -ne 0 ]]; then
  echo "::error::${STAGE}: curl exited with ${curl_status}"
  exit 1
fi

# 本文は 4xx と 5xx のときだけ出す。200 以外の 2xx と 3xx の本文は、マスクする前の JWT を含み得るため出さない。
# 中継の機器が大きな本文を返してもログを埋めないよう、本文は先頭の 4096 バイトまで出す
if [[ "${http_status}" != 200 ]]; then
  if [[ "${http_status}" == [45][0-9][0-9] ]]; then
    echo "::error::${STAGE}: HTTP status ${http_status}: $(escape_data "$(head -c 4096 -- "${response}")")"
  else
    echo "::error::${STAGE}: HTTP status ${http_status}"
  fi
  exit 1
fi

# マスクは 1 行ごとに効くため、改行を含む値や複数の値を受け取らない。
# 応答の本文には JWT が入り得るため、JSON として読めないときも jq の文言を出さない
if ! id_token="$(jq --slurp --exit-status --raw-output \
  'if length == 1 then .[0].value else empty end | strings | select(. != "" and (test("[\r\n]") | not))' \
  "${response}" 2>/dev/null)"; then
  echo "::error::${STAGE}: the response with HTTP status 200 has no single-line value"
  exit 1
fi

printf '::add-mask::%s\n' "$(escape_data "${id_token}")"
if ! printf '%s' "${id_token}" >"${OUT}"; then
  echo "::error::${STAGE}: could not write the JWT"
  exit 1
fi
