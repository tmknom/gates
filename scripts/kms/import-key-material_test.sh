#!/usr/bin/env bash
# import-key-material.sh の試験。aws をスタブに差し替え、使い捨ての鍵だけを使う
# スタブは、KMS がするように、受け取った鍵素材を使い捨てのラッピング秘密鍵で復号できる形で保存する
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# 試験が改変を検出するかを確かめるときは、改変した写しを TARGET に指定する
readonly TARGET="${TARGET:-${SCRIPT_DIR}/import-key-material.sh}"
# readonly は代入の失敗を隠すため、代入してから読み取り専用にする
ROOT="$(mktemp -d)"
readonly ROOT
trap 'rm -rf "${ROOT}"' EXIT

passed=0
failed=0

# 試験のあいだ同じ鍵を使い回し、鍵の生成の時間を減らす
readonly KEYS="${ROOT}/keys"
mkdir -p "${KEYS}"
openssl genrsa -out "${KEYS}/wrapping-private.pem" 4096 2>/dev/null
openssl pkey -in "${KEYS}/wrapping-private.pem" -pubout -outform DER -out "${KEYS}/wrapping-public.der"
# GitHub がダウンロードさせる秘密鍵と同じ PKCS#1 の PEM にする
openssl genrsa -traditional -out "${KEYS}/github-2048.pem" 2048 2>/dev/null
openssl genrsa -traditional -out "${KEYS}/github-3072.pem" 3072 2>/dev/null

# 事例ごとに、State、スタブ、ダウンロードの置き場所、TMPDIR を作り直す
setup() {
  case_dir="${ROOT}/case"
  rm -rf "${case_dir}"
  mkdir -p "${case_dir}/bin" "${case_dir}/stub" "${case_dir}/downloads" "${case_dir}/tmp"
  cat > "${case_dir}/terraform.tfstate" <<'JSON'
{"outputs":{"kms_key_arns":{"value":{"key-1":"arn:aws:kms:ap-northeast-1:123456789012:key/11111111-1111-1111-1111-111111111111","key-2":"arn:aws:kms:ap-northeast-1:123456789012:key/22222222-2222-2222-2222-222222222222"}}}}
JSON
  cp "${KEYS}/wrapping-public.der" "${case_dir}/stub/wrapping-public.der"
  cat > "${case_dir}/bin/aws" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
stub="${STUB_DIR}"
printf '%s\n' "$*" >> "${stub}/aws.log"
[ "$1" = "kms" ] || exit 2
command="$2"
shift 2
arg_value() {
  name="$1"
  shift
  while [ "$#" -gt 0 ]; do
    if [ "$1" = "${name}" ]; then
      printf '%s' "$2"
      return
    fi
    shift
  done
  exit 2
}
key_id="$(arg_value --key-id "$@")"
[ "${key_id}" = "${STUB_EXPECTED_KEY_ARN}" ] || { echo "unexpected key: ${key_id}" >&2; exit 254; }
[ "$(arg_value --region "$@")" = "ap-northeast-1" ] || exit 2
case "${command}" in
  describe-key)
    if [ -e "${stub}/encrypted-key-material.bin" ]; then
      printf '{"KeyMetadata":{"KeyState":"Enabled","Origin":"EXTERNAL","KeyUsage":"SIGN_VERIFY","KeySpec":"%s","ExpirationModel":"KEY_MATERIAL_DOES_NOT_EXPIRE"}}\n' "${STUB_KEY_SPEC}"
    else
      printf '{"KeyMetadata":{"KeyState":"%s","Origin":"%s","KeyUsage":"SIGN_VERIFY","KeySpec":"%s"}}\n' "${STUB_KEY_STATE}" "${STUB_ORIGIN}" "${STUB_KEY_SPEC}"
    fi
    ;;
  get-parameters-for-import)
    [ "$(arg_value --wrapping-algorithm "$@")" = "RSA_AES_KEY_WRAP_SHA_256" ] || exit 2
    [ "$(arg_value --wrapping-key-spec "$@")" = "RSA_4096" ] || exit 2
    printf 'import-token' > "${stub}/import-token.bin"
    printf '{"KeyId":"%s","PublicKey":"%s","ImportToken":"%s"}\n' "${key_id}" \
      "$(openssl base64 -A -in "${stub}/wrapping-public.der")" "$(openssl base64 -A -in "${stub}/import-token.bin")"
    ;;
  import-key-material)
    [ "$(arg_value --expiration-model "$@")" = "KEY_MATERIAL_DOES_NOT_EXPIRE" ] || exit 2
    token="$(arg_value --import-token "$@")"
    cmp -s "${token#fileb://}" "${stub}/import-token.bin" || { echo "wrong import token" >&2; exit 254; }
    material="$(arg_value --encrypted-key-material "$@")"
    cp "${material#fileb://}" "${stub}/encrypted-key-material.bin"
    printf '{"KeyId":"%s"}\n' "${key_id}"
    ;;
  get-public-key)
    printf '{"KeyId":"%s","PublicKey":"%s"}\n' "${key_id}" "$(openssl base64 -A -in "${stub}/kms-public.der")"
    ;;
  *)
    exit 2
    ;;
esac
STUB
  chmod 755 "${case_dir}/bin/aws"
  export STUB_DIR="${case_dir}/stub"
  export STUB_EXPECTED_KEY_ARN="arn:aws:kms:ap-northeast-1:123456789012:key/22222222-2222-2222-2222-222222222222"
  export STUB_KEY_STATE="PendingImport"
  export STUB_ORIGIN="EXTERNAL"
  export STUB_KEY_SPEC="RSA_2048"
}

# 秘密鍵を、運用者がダウンロードした鍵の置き場所に写す
download_key() {
  cp "$1" "$2"
  # インポートしたときの KMS の公開鍵は、ダウンロードした鍵の公開鍵になる
  openssl pkey -in "$1" -pubout -outform DER -out "${case_dir}/stub/kms-public.der"
}

run_target() {
  set +e
  # 運用者のシェルが本物の秘密鍵のパスを持っていても、試験がそれを読んで消さないよう外す
  env -u GITHUB_PRIVATE_KEY_PATH -u KMS_KEY_NAME PATH="${case_dir}/bin:${PATH}" TMPDIR="${case_dir}/tmp" \
    TF_STATE_PATH="${case_dir}/terraform.tfstate" \
    "$@" "${TARGET}" < /dev/null > "${case_dir}/stdout" 2> "${case_dir}/stderr"
  status=$?
  set -e
}

pass() {
  passed=$((passed + 1))
  printf 'ok   %s\n' "$1"
}

fail() {
  failed=$((failed + 1))
  printf 'FAIL %s: %s\n' "$1" "$2"
  sed 's/^/     stderr: /' "${case_dir}/stderr"
}

tmp_is_empty() {
  [ -z "$(ls -A "${case_dir}/tmp")" ]
}

# KMS が受け取った鍵素材を、使い捨てのラッピング秘密鍵で戻す
unwrap_imported() {
  material="${case_dir}/stub/encrypted-key-material.bin"
  head -c 512 "${material}" > "${case_dir}/aes-key-wrapped.bin"
  tail -c +513 "${material}" > "${case_dir}/key-material-wrapped.bin"
  openssl pkeyutl -decrypt -inkey "${KEYS}/wrapping-private.pem" \
    -pkeyopt rsa_padding_mode:oaep -pkeyopt rsa_oaep_md:sha256 -pkeyopt rsa_mgf1_md:sha256 \
    -in "${case_dir}/aes-key-wrapped.bin" -out "${case_dir}/aes-key.bin"
  openssl enc -d -id-aes256-wrap-pad -K "$(xxd -p -c 32 "${case_dir}/aes-key.bin")" -iv A65959A6 \
    -in "${case_dir}/key-material-wrapped.bin" -out "${case_dir}/key-material.der"
}

# 失敗の事例は、終了コードだけでなく、意図した検査で止まったことをメッセージで確かめる
stopped_by() {
  grep -Fq "$1" "${case_dir}/stderr"
}

fingerprint_of() {
  openssl pkey -in "$1" -pubout -outform DER | openssl sha256 -binary | openssl base64
}

test_imports_the_downloaded_key() {
  name="imports the key given by GITHUB_PRIVATE_KEY_PATH, removes it, and prints the fingerprint"
  setup
  key="${case_dir}/downloads/app.2026-11-15.private-key.pem"
  download_key "${KEYS}/github-2048.pem" "${key}"
  openssl pkcs8 -topk8 -nocrypt -in "${key}" -outform DER -out "${case_dir}/expected.der"
  run_target KMS_KEY_NAME=key-2 GITHUB_PRIVATE_KEY_PATH="${key}"
  if [ "${status}" -ne 0 ]; then fail "${name}" "exit ${status}"; return; fi
  if [ -e "${key}" ]; then fail "${name}" "the private key remains"; return; fi
  if ! tmp_is_empty; then fail "${name}" "the working directory remains"; return; fi
  unwrap_imported
  if ! cmp -s "${case_dir}/key-material.der" "${case_dir}/expected.der"; then fail "${name}" "the key material differs"; return; fi
  if ! grep -Fq "SHA256:$(fingerprint_of "${KEYS}/github-2048.pem")" "${case_dir}/stdout"; then fail "${name}" "no fingerprint"; return; fi
  pass "${name}"
}

test_requires_the_private_key_path() {
  name="requires GITHUB_PRIVATE_KEY_PATH before calling AWS, without looking for the key elsewhere"
  setup
  download_key "${KEYS}/github-2048.pem" "${case_dir}/downloads/app.private-key.pem"
  run_target KMS_KEY_NAME=key-2 HOME="${case_dir}"
  if [ "${status}" -eq 0 ]; then fail "${name}" "exit 0"; return; fi
  if ! stopped_by "GITHUB_PRIVATE_KEY_PATH must be the path of the downloaded private key"; then fail "${name}" "not stopped by the path check"; return; fi
  if [ -e "${case_dir}/stub/aws.log" ]; then fail "${name}" "called AWS"; return; fi
  if [ ! -e "${case_dir}/downloads/app.private-key.pem" ]; then fail "${name}" "removed a key it was not given"; return; fi
  pass "${name}"
}

test_rejects_a_wrong_key_length() {
  name="rejects a key whose length differs from the key spec, removes it, and imports nothing"
  setup
  key="${case_dir}/downloads/app.private-key.pem"
  download_key "${KEYS}/github-3072.pem" "${key}"
  run_target KMS_KEY_NAME=key-2 GITHUB_PRIVATE_KEY_PATH="${key}"
  if [ "${status}" -eq 0 ]; then fail "${name}" "exit 0"; return; fi
  if [ -e "${key}" ]; then fail "${name}" "the private key remains"; return; fi
  if grep -qs '^kms import-key-material' "${case_dir}/stub/aws.log"; then fail "${name}" "imported"; return; fi
  if ! tmp_is_empty; then fail "${name}" "the working directory remains"; return; fi
  if ! stopped_by "3072 bit, but the KMS key expects RSA_2048"; then fail "${name}" "not stopped by the key length check"; return; fi
  if ! stopped_by "Delete the key you generated"; then fail "${name}" "no request to delete the key on GitHub"; return; fi
  pass "${name}"
}

test_rejects_a_key_not_pending_import() {
  name="rejects a KMS key that is not PendingImport before reading the private key"
  setup
  export STUB_KEY_STATE="Enabled"
  key="${case_dir}/downloads/app.private-key.pem"
  download_key "${KEYS}/github-2048.pem" "${key}"
  run_target KMS_KEY_NAME=key-2 GITHUB_PRIVATE_KEY_PATH="${key}"
  if [ "${status}" -eq 0 ]; then fail "${name}" "exit 0"; return; fi
  if ! stopped_by "must be PendingImport, but is Enabled"; then fail "${name}" "not stopped by the key state check"; return; fi
  if [ ! -e "${key}" ]; then fail "${name}" "removed the private key before reading it"; return; fi
  if grep -qs '^kms get-parameters-for-import' "${case_dir}/stub/aws.log"; then fail "${name}" "prepared the import"; return; fi
  pass "${name}"
}

test_rejects_a_key_not_external() {
  name="rejects a KMS key whose origin is not EXTERNAL"
  setup
  export STUB_ORIGIN="AWS_KMS"
  key="${case_dir}/downloads/app.private-key.pem"
  download_key "${KEYS}/github-2048.pem" "${key}"
  run_target KMS_KEY_NAME=key-2 GITHUB_PRIVATE_KEY_PATH="${key}"
  if [ "${status}" -eq 0 ]; then fail "${name}" "exit 0"; return; fi
  if ! stopped_by "origin must be EXTERNAL, but is AWS_KMS"; then fail "${name}" "not stopped by the origin check"; return; fi
  if grep -qs '^kms get-parameters-for-import' "${case_dir}/stub/aws.log"; then fail "${name}" "prepared the import"; return; fi
  pass "${name}"
}

test_rejects_an_invalid_name() {
  name="rejects a KMS_KEY_NAME that is not the form key-N before calling AWS"
  setup
  for value in "" "key-0" "key-1;true" "$(printf 'key-1\nkey-2')" "../key-1"; do
    run_target KMS_KEY_NAME="${value}"
    if [ "${status}" -eq 0 ]; then fail "${name}" "accepted '${value}'"; return; fi
    if ! stopped_by "KMS_KEY_NAME must be"; then fail "${name}" "'${value}' not stopped by the name check"; return; fi
  done
  if [ -e "${case_dir}/stub/aws.log" ]; then fail "${name}" "called AWS"; return; fi
  pass "${name}"
}

test_rejects_an_unknown_name() {
  name="rejects a name that is not in kms_key_arns before calling AWS"
  setup
  key="${case_dir}/downloads/app.private-key.pem"
  download_key "${KEYS}/github-2048.pem" "${key}"
  run_target KMS_KEY_NAME=key-3 GITHUB_PRIVATE_KEY_PATH="${key}"
  if [ "${status}" -eq 0 ]; then fail "${name}" "exit 0"; return; fi
  if ! stopped_by "no KMS key named key-3"; then fail "${name}" "not stopped by the state lookup"; return; fi
  if [ -e "${case_dir}/stub/aws.log" ]; then fail "${name}" "called AWS"; return; fi
  pass "${name}"
}

test_keeps_a_file_that_is_not_a_private_key() {
  name="keeps a file that is not a private key and imports nothing"
  setup
  other="${case_dir}/downloads/notes.txt"
  printf 'not a key\n' > "${other}"
  public="${case_dir}/downloads/public.pem"
  openssl pkey -in "${KEYS}/github-2048.pem" -pubout -out "${public}"
  for file in "${other}" "${public}"; do
    run_target KMS_KEY_NAME=key-2 GITHUB_PRIVATE_KEY_PATH="${file}"
    if [ "${status}" -eq 0 ]; then fail "${name}" "accepted ${file}"; return; fi
    if ! stopped_by "is not an RSA private key"; then fail "${name}" "${file} not stopped by the key check"; return; fi
    if [ ! -e "${file}" ]; then fail "${name}" "removed ${file}"; return; fi
  done
  if grep -qs '^kms import-key-material' "${case_dir}/stub/aws.log"; then fail "${name}" "imported"; return; fi
  if ! tmp_is_empty; then fail "${name}" "the working directory remains"; return; fi
  pass "${name}"
}

test_rejects_a_symbolic_link() {
  name="rejects a symbolic link to the private key before calling AWS, because removing the link would leave the key"
  setup
  key="${case_dir}/downloads/app.private-key.pem"
  download_key "${KEYS}/github-2048.pem" "${key}"
  link="${case_dir}/downloads/link.private-key.pem"
  ln -s "${key}" "${link}"
  run_target KMS_KEY_NAME=key-2 GITHUB_PRIVATE_KEY_PATH="${link}"
  if [ "${status}" -eq 0 ]; then fail "${name}" "exit 0"; return; fi
  if ! stopped_by "is a symbolic link"; then fail "${name}" "not stopped by the link check"; return; fi
  if [ ! -L "${link}" ] || [ ! -e "${key}" ]; then fail "${name}" "removed the link or the key"; return; fi
  if [ -e "${case_dir}/stub/aws.log" ]; then fail "${name}" "called AWS"; return; fi
  pass "${name}"
}

test_fails_when_kms_returns_another_public_key() {
  name="fails when the public key of the KMS key differs from the imported key"
  setup
  key="${case_dir}/downloads/app.private-key.pem"
  download_key "${KEYS}/github-2048.pem" "${key}"
  openssl pkey -in "${KEYS}/github-3072.pem" -pubout -outform DER -out "${case_dir}/stub/kms-public.der"
  run_target KMS_KEY_NAME=key-2 GITHUB_PRIVATE_KEY_PATH="${key}"
  if [ "${status}" -eq 0 ]; then fail "${name}" "exit 0"; return; fi
  if ! stopped_by "does not match the downloaded private key"; then fail "${name}" "not stopped by the fingerprint check"; return; fi
  if [ -e "${key}" ]; then fail "${name}" "the private key remains"; return; fi
  if ! tmp_is_empty; then fail "${name}" "the working directory remains"; return; fi
  pass "${name}"
}

test_imports_the_downloaded_key
test_requires_the_private_key_path
test_rejects_a_wrong_key_length
test_rejects_a_key_not_pending_import
test_rejects_a_key_not_external
test_rejects_an_invalid_name
test_rejects_an_unknown_name
test_keeps_a_file_that_is_not_a_private_key
test_rejects_a_symbolic_link
test_fails_when_kms_returns_another_public_key

printf '%d passed, %d failed\n' "${passed}" "${failed}"
[ "${failed}" -eq 0 ]
