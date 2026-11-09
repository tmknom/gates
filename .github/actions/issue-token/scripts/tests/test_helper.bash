# 試験に共通する準備と確認
# shellcheck disable=SC2154 # output と stderr は bats の run が設定する

# shellcheck disable=SC2034 # 試験のファイルが使う
SCRIPTS_DIR="$(cd "${BATS_TEST_DIRNAME}/.." && pwd)"

# 試験ごとの作業ディレクトリを作り、スタブを PATH の先頭に置く。
# 確認には本物の jq を REAL_JQ で使い、スタブの記録に試験自身の呼び出しを混ぜない
setup_action_test() {
  bats_require_minimum_version 1.5.0
  REAL_JQ="$(command -v jq)"
  REAL_RM="$(command -v rm)"
  export REAL_JQ REAL_RM
  export STUB_LOG_DIR="${BATS_TEST_TMPDIR}/stub-log"
  mkdir -p "${STUB_LOG_DIR}"
  PATH="${BATS_TEST_DIRNAME}/stubs:${PATH}"
  WORK="${BATS_TEST_TMPDIR}/work"
  mkdir -m 700 "${WORK}"
}

# コマンドが失敗することを確かめる。
# bash の `!` を付けたコマンドは errexit の対象外で、失敗させたい箇所で試験が通ってしまうため使う
refute() {
  if "$@"; then
    echo "expected to fail: $*" >&2
    return 1
  fi
}

# 所有者だけが読み書きできる権限であることを確かめる
assert_owner_only() {
  [[ "$(stat -c %a "$1")" == 600 ]]
}

# 文字列が、標準出力、標準エラー、スタブが記録した引数のどこにも現れないことを確かめる。
# 第 2 引数を与えると、その行と完全に一致する標準出力の行だけを許す
assert_not_exposed() {
  local secret="$1" allowed="${2-}"
  local line
  while IFS= read -r line; do
    if [[ "${line}" == *"${secret}"* && "${line}" != "${allowed}" ]]; then
      echo "exposed in stdout: ${line}" >&2
      return 1
    fi
  done <<<"${output}"
  if [[ "${stderr}" == *"${secret}"* ]]; then
    echo "exposed in stderr" >&2
    return 1
  fi
  local log
  for log in "${STUB_LOG_DIR}"/*.args; do
    [[ -e "${log}" ]] || continue
    if grep -qF -- "${secret}" "${log}"; then
      echo "exposed in arguments: ${log}" >&2
      return 1
    fi
  done
}
