package audit

import (
	"log/slog"

	"github.com/tmknom/gates/internal/errorcode"
	"github.com/tmknom/gates/internal/nonzero"
)

// success は、成功した発行処理の結果の値である。失敗の結果の値はエラーコードである。
const success = "success"

// Result は、発行処理の結果と、その結果で終わったときだけ持つ値（拒否の理由と失効の結果）である。
// 結果に伴う値を結果と一緒に作り、成功の記録に拒否の理由や失効の結果が付かないようにする。
// どのエラーコードが照合失敗かはエラーコードを定める側が知るため、照合失敗と失効の結果の対応は検査しない。
type Result struct {
	succeeded  bool
	code       errorcode.Code
	reason     string
	revocation RevocationOutcome
}

// IsZero は、Succeeded と Failed を経ていないゼロ値かを返す。Failed はゼロ値のエラーコードを受け付けないため、成功でもなくエラーコードも持たない値はゼロ値に限る。
func (r Result) IsZero() bool {
	return !r.succeeded && r.code.IsZero()
}

// Succeeded は、成功の結果を返す。
func Succeeded() Result {
	return Result{succeeded: true}
}

// Failed は、エラーコードと拒否の理由を持つ失敗の結果を返す。
func Failed(code errorcode.Code, reason string) Result {
	// ゼロ値はエラーコードを持たず、失敗の結果の値を書けない
	nonzero.Require(code)
	return Result{code: code, reason: reason}
}

// VerificationFailed は、照合失敗の結果を、失効の結果とともに返す。
func VerificationFailed(code errorcode.Code, reason string, outcome RevocationOutcome) Result {
	// ゼロ値は Token Issuer の失効の結果を写していない値で、どの失効の結果として記録しても事実と食い違う
	nonzero.Require(outcome)
	result := Failed(code, reason)
	result.revocation = outcome
	return result
}

func (r Result) attrs() []any {
	// 結果のない監査記録は、発行処理がどう終わったかを示せない
	nonzero.Require(r)
	if r.succeeded {
		return []any{slog.String("result", success)}
	}
	// 失敗は拒否の理由を記録する条件を満たすため、値が空でもキーを書き、条件を満たさなかった記録と見分けられるようにする
	attrs := []any{slog.String("result", r.code.Value()), slog.String("reason", r.reason)}
	if !r.revocation.IsZero() {
		attrs = append(attrs, slog.String("revocation", r.revocation.value))
	}
	return attrs
}
