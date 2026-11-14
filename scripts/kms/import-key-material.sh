#!/usr/bin/env bash
# GitHub App Private Key を、Terraform が作った PendingImport の KMS Key に鍵素材としてインポートする
# 手順と、ラップのアルゴリズムを選んだ理由は docs/contributing/import-key-material.md にある
# 秘密鍵を扱うため、運用者が手元のシェルで実行する。Coding Agent は実行しない
set -euo pipefail
# 作業ディレクトリと、その中の鍵素材を、ほかのユーザーから読めなくする
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
TF_STATE_PATH="${TF_STATE_PATH:-${REPO_ROOT}/terraform/local/terraform.tfstate}"
readonly SCRIPT_DIR REPO_ROOT TF_STATE_PATH

info() {
  printf '%s\n' "$*" >&2
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

work_dir=""
# RSA の秘密鍵として読めたファイルだけを入れる。打ち間違えたパスのファイルを消さないため
private_key_path=""
imported="false"

cleanup() {
  status=$?
  if [ -n "${work_dir}" ]; then
    rm -rf "${work_dir}"
  fi
  if [ -n "${private_key_path}" ] && [ -e "${private_key_path}" ]; then
    rm -f "${private_key_path}"
    info "Removed the downloaded private key: ${private_key_path}"
  fi
  if [ "${status}" -ne 0 ] && [ -n "${private_key_path}" ] && [ "${imported}" = "false" ]; then
    info "The key material was not imported. Delete the key you generated from the GitHub App Key pairs."
  fi
}
trap cleanup EXIT
# 中断でも EXIT の trap を通し、秘密鍵と作業ディレクトリを消す
trap 'exit 130' INT TERM HUP

# grep は行ごとに判定するため、許さない文字を先に case で拒否する
case "${KMS_KEY_NAME:-}" in
  "" | *[!a-z0-9-]*) fail "KMS_KEY_NAME must be the name of the key in kms_keys, such as key-2" ;;
esac
printf '%s\n' "${KMS_KEY_NAME}" | grep -Eqx 'key-[1-9][0-9]{0,8}' ||
  fail "KMS_KEY_NAME must be the name of the key in kms_keys, such as key-2"
# 秘密鍵は運用者が指定したファイルだけを扱い、ほかの場所を探さない
[ -n "${GITHUB_PRIVATE_KEY_PATH:-}" ] ||
  fail "GITHUB_PRIVATE_KEY_PATH must be the path of the downloaded private key"
# rm はリンクだけを消し、リンク先の秘密鍵を残すため、リンクを受け付けない
[ ! -L "${GITHUB_PRIVATE_KEY_PATH}" ] ||
  fail "${GITHUB_PRIVATE_KEY_PATH} is a symbolic link: give the path of the downloaded file itself"

for command in aws jq openssl xxd; do
  command -v "${command}" > /dev/null 2>&1 || fail "${command} is required"
done
# macOS 標準の LibreSSL には -id-aes256-wrap-pad がない
case "$(openssl version)" in
  "OpenSSL "[3-9].* | "OpenSSL "[1-9][0-9].*) ;;
  *) fail "OpenSSL 3 or later is required, but found: $(openssl version)" ;;
esac

[ -f "${TF_STATE_PATH}" ] || fail "the terraform state is not found: ${TF_STATE_PATH}"
kms_key_arn="$(jq -r --arg name "${KMS_KEY_NAME}" '.outputs.kms_key_arns.value[$name] // empty' "${TF_STATE_PATH}")"
[ -n "${kms_key_arn}" ] ||
  fail "no KMS key named ${KMS_KEY_NAME} in the output kms_key_arns: add it to kms_keys and run make tf-plan and make tf-apply"
printf '%s\n' "${kms_key_arn}" | grep -Eqx 'arn:aws:kms:[a-z0-9-]+:[0-9]{12}:key/[0-9a-f-]+' ||
  fail "the output kms_key_arns has an unexpected value for ${KMS_KEY_NAME}"
region="$(printf '%s' "${kms_key_arn}" | cut -d: -f4)"
info "KMS key: ${KMS_KEY_NAME} ${kms_key_arn}"

# 鍵素材のない、署名用の外部の鍵だけを対象にする。インポート済みの鍵の鍵素材は交換できない
metadata="$(aws kms describe-key --region "${region}" --key-id "${kms_key_arn}" --output json)"
key_state="$(printf '%s' "${metadata}" | jq -r '.KeyMetadata.KeyState')"
origin="$(printf '%s' "${metadata}" | jq -r '.KeyMetadata.Origin')"
key_usage="$(printf '%s' "${metadata}" | jq -r '.KeyMetadata.KeyUsage')"
key_spec="$(printf '%s' "${metadata}" | jq -r '.KeyMetadata.KeySpec')"
[ "${key_state}" = "PendingImport" ] || fail "the KMS key must be PendingImport, but is ${key_state}"
[ "${origin}" = "EXTERNAL" ] || fail "the KMS key origin must be EXTERNAL, but is ${origin}"
[ "${key_usage}" = "SIGN_VERIFY" ] || fail "the KMS key usage must be SIGN_VERIFY, but is ${key_usage}"
case "${key_spec}" in
  RSA_2048 | RSA_3072 | RSA_4096) expected_bits="${key_spec#RSA_}" ;;
  *) fail "the KMS key spec must be RSA_2048, RSA_3072, or RSA_4096, but is ${key_spec}" ;;
esac

# 秘密鍵を読む前に、インポートの準備を済ませる
# macOS の mktemp はテンプレートがないと TMPDIR を使わないため、置き場所を明示する
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/gates-import-key-material.XXXXXXXX")"

aws kms get-parameters-for-import --region "${region}" --key-id "${kms_key_arn}" \
  --wrapping-algorithm RSA_AES_KEY_WRAP_SHA_256 --wrapping-key-spec RSA_4096 \
  --output json > "${work_dir}/import-parameters.json"
jq -r .PublicKey "${work_dir}/import-parameters.json" | openssl base64 -d -A -out "${work_dir}/wrapping-public-key.der"
jq -r .ImportToken "${work_dir}/import-parameters.json" | openssl base64 -d -A -out "${work_dir}/import-token.bin"
openssl rand -out "${work_dir}/aes-key.bin" 32
openssl pkeyutl -encrypt -pubin -keyform DER -inkey "${work_dir}/wrapping-public-key.der" \
  -pkeyopt rsa_padding_mode:oaep -pkeyopt rsa_oaep_md:sha256 -pkeyopt rsa_mgf1_md:sha256 \
  -in "${work_dir}/aes-key.bin" -out "${work_dir}/aes-key-wrapped.bin"

candidate="${GITHUB_PRIVATE_KEY_PATH}"
[ -f "${candidate}" ] || fail "the private key is not found: ${candidate}"

# 公開鍵の部分だけを出す。-text_pub は秘密鍵の成分を出力しない
public_text="$(openssl pkey -in "${candidate}" -noout -text_pub 2> /dev/null)" ||
  fail "${candidate} is not an RSA private key"
printf '%s\n' "${public_text}" | grep -q '^Modulus:' || fail "${candidate} is not an RSA private key"
private_key_path="${candidate}"

bits="$(printf '%s\n' "${public_text}" | sed -n 's/^Public-Key: (\([0-9]*\) bit)$/\1/p')"
[ "${bits}" = "${expected_bits}" ] ||
  fail "the private key is ${bits} bit, but the KMS key expects ${key_spec}: see Key Rotation in docs/contributing/rotate-key.md"
info "Key length: ${bits} bit"
local_fingerprint="$(openssl pkey -in "${private_key_path}" -pubout -outform DER | openssl sha256 -binary | openssl base64)"

# GitHub がダウンロードさせる PKCS#1 の PEM を、KMS が求める PKCS#8 の DER にする
# -id-aes256-wrap-pad は標準入力を受け付けないため、変換した鍵素材はいったんファイルに書く
openssl pkcs8 -topk8 -nocrypt -in "${private_key_path}" -outform DER -out "${work_dir}/private-key.der"
openssl enc -id-aes256-wrap-pad -K "$(xxd -p -c 32 "${work_dir}/aes-key.bin")" -iv A65959A6 \
  -in "${work_dir}/private-key.der" -out "${work_dir}/key-material-wrapped.bin"
cat "${work_dir}/aes-key-wrapped.bin" "${work_dir}/key-material-wrapped.bin" > "${work_dir}/encrypted-key-material.bin"

aws kms import-key-material --region "${region}" --key-id "${kms_key_arn}" \
  --encrypted-key-material "fileb://${work_dir}/encrypted-key-material.bin" \
  --import-token "fileb://${work_dir}/import-token.bin" \
  --expiration-model KEY_MATERIAL_DOES_NOT_EXPIRE > /dev/null
imported="true"
info "Imported the key material."

rm -rf "${work_dir}"
work_dir=""
rm -f "${private_key_path}"
info "Removed the downloaded private key and the working directory: ${private_key_path}"
private_key_path=""

metadata="$(aws kms describe-key --region "${region}" --key-id "${kms_key_arn}" --output json)"
key_state="$(printf '%s' "${metadata}" | jq -r '.KeyMetadata.KeyState')"
expiration_model="$(printf '%s' "${metadata}" | jq -r '.KeyMetadata.ExpirationModel')"
[ "${key_state}" = "Enabled" ] || fail "the KMS key must be Enabled after the import, but is ${key_state}"
[ "${expiration_model}" = "KEY_MATERIAL_DOES_NOT_EXPIRE" ] ||
  fail "the key material must not expire, but the expiration model is ${expiration_model}"

# KMS は公開鍵を SubjectPublicKeyInfo の DER で返す。GitHub も同じ DER の SHA-256 を表示する
kms_fingerprint="$(aws kms get-public-key --region "${region}" --key-id "${kms_key_arn}" --output json |
  jq -r .PublicKey | openssl base64 -d -A | openssl sha256 -binary | openssl base64)"
[ "${kms_fingerprint}" = "${local_fingerprint}" ] ||
  fail "the public key of the KMS key does not match the downloaded private key"

printf 'KMS key:     %s %s\n' "${KMS_KEY_NAME}" "${kms_key_arn}"
printf 'Key state:   %s %s\n' "${key_state}" "${expiration_model}"
printf 'Fingerprint: SHA256:%s\n' "${kms_fingerprint}"
printf 'Check that the GitHub App Key pairs show a key with this fingerprint.\n'
