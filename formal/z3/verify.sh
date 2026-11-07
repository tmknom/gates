#!/bin/sh
# 検査ごとに、モデルと検査を連結して独立した Z3 プロセスで解き、検査の先頭の「; expect:」の判定と比べる。
# 1 つの検査に check-sat が複数あるときは、「; expect:」に判定を空白で区切って同じ順に並べる。
# 必要なのは sh、z3、GNU timeout、find、sort、sed、grep、wc、tr だけで、scripts/z3 のコンテナで実行する。
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
cat \
    model/values.smt2 \
    model/standard.smt2 \
    model/github.smt2 \
    model/gates-current.smt2 \
    model/gates-proposals.smt2 \
    checks/fixtures.smt2 > "$scratch/model.smt2"
find checks -name '*.smt2' ! -name fixtures.smt2 | sort > "$scratch/checks"
# check-sat ごとのソルバーのタイムアウト（ミリ秒）をモデルから読み、プロセスの上限をこれに合わせる
solver_ms=$(sed -n 's/^(set-option :timeout \([0-9][0-9]*\))$/\1/p' model/values.smt2)
if [ -z "$solver_ms" ]; then
    echo 'ERROR: model/values.smt2 must set the solver timeout with (set-option :timeout <ms>).' >&2
    exit 2
fi
solver_seconds=$(((solver_ms + 999) / 1000))
pass=0
fail=0
while IFS= read -r check; do
    line=$(sed -n '1s/^; expect: //p' "$check")
    valid=yes
    expected=
    count=0
    # 判定を 1 つの空白で区切り直し、空白の数に依らず z3 の出力と比べられるようにする
    for verdict in $line; do
        case "$verdict" in
            sat|unsat)
                expected="${expected:+$expected }$verdict"
                count=$((count + 1))
                ;;
            *) valid=no ;;
        esac
    done
    if [ -z "$expected" ]; then
        valid=no
    fi
    if [ "$valid" = no ]; then
        fail=$((fail + 1))
        printf 'FAIL %s: the first line must be "; expect:" followed by sat or unsat for each check-sat\n' "$check" >&2
        continue
    fi
    queries=$(grep -o '(check-sat)' "$check" | wc -l)
    queries=$((queries))
    if [ "$count" -ne "$queries" ]; then
        fail=$((fail + 1))
        printf 'FAIL %s: "; expect:" has %s verdicts but the check has %s check-sat\n' "$check" "$count" "$queries" >&2
        continue
    fi
    # 各 check-sat がソルバーのタイムアウトまで使っても、unknown を出す前にプロセスが打ち切られないようにする
    # 1 回分の余裕は、モデルの読み込みに充てる
    limit=$((solver_seconds * (queries + 1)))
    cat "$scratch/model.smt2" "$check" > "$scratch/input.smt2"
    code=0
    timeout "${limit}s" z3 -in < "$scratch/input.smt2" > "$scratch/out" 2> "$scratch/err" || code=$?
    actual=$(tr '\n' ' ' < "$scratch/out" | sed 's/ *$//')
    if [ "$code" -eq 124 ]; then
        actual=timeout
    elif [ "$code" -ne 0 ]; then
        actual=error
    else
        for verdict in $actual; do
            case "$verdict" in
                sat|unsat|unknown) : ;;
                *) actual=error ;;
            esac
        done
    fi
    if [ "$actual" = "$expected" ]; then
        pass=$((pass + 1))
    else
        fail=$((fail + 1))
        printf 'FAIL %s: expected=%s actual=%s\n' "$check" "$expected" "$actual" >&2
        if [ "$actual" = error ]; then
            cat "$scratch/out" "$scratch/err" >&2
        fi
    fi
done < "$scratch/checks"
if [ "$fail" -ne 0 ]; then
    printf 'FAIL: %s matched; %s failures\n' "$pass" "$fail" >&2
    exit 1
fi
if [ "$pass" -eq 0 ]; then
    echo 'FAIL: no checks found' >&2
    exit 1
fi
printf 'PASS: %s/%s checks match expected SAT/UNSAT results.\n' "$pass" "$pass"
