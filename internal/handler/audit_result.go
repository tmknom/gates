package handler

import (
	"errors"

	"github.com/tmknom/gates/internal/audit"
	"github.com/tmknom/gates/internal/errorcode"
	"github.com/tmknom/gates/internal/revocation"
	"github.com/tmknom/gates/internal/tokenissuer"
)

// failedResult は、部品が返した失敗から、エラーコードと拒否の理由を持つ監査記録の結果を作る。
// 失効の結果は、Domain Model の「Audit Record」の記録する条件に従い、照合失敗のエラーコードで終わる場合だけ持つ。
func failedResult(code errorcode.Code, err error) audit.Result {
	if code != errorcode.IssuanceVerificationFailed {
		return audit.Failed(code, err.Error())
	}
	// 失効の結果を持たない照合失敗はゼロ値のまま渡し、audit.VerificationFailed の panic で止める
	var outcome revocation.Outcome
	if failure, ok := errors.AsType[*tokenissuer.VerificationFailure](err); ok {
		outcome = failure.RevocationOutcome()
	}
	return audit.VerificationFailed(code, err.Error(), outcome)
}
