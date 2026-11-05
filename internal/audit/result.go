package audit

import "log/slog"

// success は、成功した発行処理の結果の値である。失敗の結果の値はエラーコードである。
const success = "success"

// Result は、発行処理の結果と、その結果で終わったときだけ持つ値（拒否の理由と失効の結果）である。
// 結果に伴う値を結果と一緒に作り、成功の記録に拒否の理由や失効の結果が付かないようにする。
// どのエラーコードが照合失敗かはエラーコードを定める側が知るため、照合失敗と失効の結果の対応は検査しない。
// ゼロ値は結果を持たず、Write に渡すと panic する。
type Result struct {
	value      string
	reason     string
	revocation RevocationOutcome
}

// Succeeded は、成功の結果を返す。
func Succeeded() Result {
	return Result{value: success}
}

// Failed は、エラーコードと拒否の理由を持つ失敗の結果を返す。
// 空の code と reason は、Lambda Design が監査記録に空文字列を書かないと定めるため、不変条件の破れとして panic する。
// 成功の値と同じ code も、失敗が成功として読まれるため panic する。
func Failed(code, reason string) Result {
	if code == "" || code == success || reason == "" {
		panic("failed result requires an error code other than success and a reason")
	}
	return Result{value: code, reason: reason}
}

// VerificationFailed は、照合失敗の結果を、失効の結果とともに返す。3 つの値以外の失効の結果は panic する。
func VerificationFailed(code, reason string, outcome RevocationOutcome) Result {
	if !outcome.valid() {
		panic("unknown revocation outcome")
	}
	result := Failed(code, reason)
	result.revocation = outcome
	return result
}

func (r Result) attrs() []any {
	if r.value == "" {
		panic("audit record requires a result")
	}
	attrs := []any{slog.String("result", r.value)}
	if r.reason != "" {
		attrs = append(attrs, slog.String("reason", r.reason))
	}
	if r.revocation != "" {
		attrs = append(attrs, slog.String("revocation", string(r.revocation)))
	}
	return attrs
}
