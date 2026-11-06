#!/bin/sh
# 検査ごとに、モデルと検査を連結して独立した Z3 プロセスで解き、期待する判定と比べる。
# 必要なのは sh、z3、GNU timeout、cut だけで、scripts/z3 のコンテナで実行する。
set -eu
cd "$(dirname "$0")"
if ! command -v z3 >/dev/null 2>&1; then
    echo 'ERROR: z3 CLI is required.' >&2
    exit 2
fi
if ! command -v timeout >/dev/null 2>&1; then
    echo 'ERROR: GNU timeout is required.' >&2
    exit 2
fi
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT HUP INT TERM
pass=0
fail=0
while IFS='|' read -r id relative expected; do
    [ -n "$id" ] || continue
    input="$scratch/$id.smt2"
    cat \
        model/values.smt2 \
        model/projection.smt2 \
        model/standard.smt2 \
        model/github.smt2 \
        model/gates-current.smt2 \
        model/gates-proposals.smt2 \
        model/audit.smt2 \
        checks/fixtures.smt2 > "$input"
    case "$id" in
      OBS??OK|OBS??AUD)
        number=$(printf '%s' "$id" | cut -c 4-5)
        cat "checks/observations/fixture$number.smt2" >> "$input"
        ;;
    esac
    cat "$relative" >> "$input"
    code=0
    timeout 16s z3 -in < "$input" > "$scratch/out" 2> "$scratch/err" || code=$?
    actual=$(cat "$scratch/out")
    if [ "$code" -eq 124 ]; then
        actual=timeout
    elif [ "$code" -ne 0 ]; then
        actual=error
    else
        case "$actual" in
            sat|unsat|unknown) : ;;
            *) actual=error ;;
        esac
    fi
    if [ "$actual" = "$expected" ]; then
        pass=$((pass + 1))
    else
        fail=$((fail + 1))
        printf 'FAIL %s: expected=%s actual=%s\n' "$id" "$expected" "$actual" >&2
        if [ "$actual" = error ]; then
            cat "$scratch/out" "$scratch/err" >&2
        fi
    fi
done < checks/expected.txt
if [ "$fail" -ne 0 ]; then
    printf 'FAIL: %s matched; %s failures\n' "$pass" "$fail" >&2
    exit 1
fi
printf 'PASS: %s/%s checks match expected SAT/UNSAT results.\n' "$pass" "$pass"
