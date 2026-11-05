package handler

import (
	"errors"

	"github.com/tmknom/gates/internal/audit"
	"github.com/tmknom/gates/internal/errorcode"
	"github.com/tmknom/gates/internal/issuer"
)

// reasonPanicRecovered は、panic を回復した発行処理の拒否の理由である。panic の値を含めないため、固定の文言にする。
const reasonPanicRecovered = "panic recovered"

// revocationOutcomes は、Token Issuer の失効の結果を、監査記録の失効の結果に写す。
// 写していない値はゼロ値になり、audit.VerificationFailed が panic する。既定の値に倒すと、写し漏れを事実と異なる失効の結果として記録するためである。
// switch にしないのは、3 つの値のどれでもない場合の return に到達できないためである。
var revocationOutcomes = map[issuer.RevocationOutcome]audit.RevocationOutcome{
	issuer.RevocationNotAttempted: audit.RevocationNotAttempted,
	issuer.RevocationSucceeded:    audit.RevocationSucceeded,
	issuer.RevocationFailed:       audit.RevocationFailed,
}

// failedResult は、部品が返した失敗から、エラーコードと拒否の理由を持つ監査記録の結果を作る。
// 失効の結果は、Domain Model の「Audit Record」の記録する条件に従い、照合失敗のエラーコードで終わる場合だけ持つ。
func failedResult(code errorcode.Code, err error) audit.Result {
	if code != errorcode.IssuanceVerificationFailed {
		return audit.Failed(code, err.Error())
	}
	// 失効の結果を持たない照合失敗はゼロ値のまま渡し、audit.VerificationFailed の panic で止める
	var outcome audit.RevocationOutcome
	if failure, ok := errors.AsType[*issuer.VerificationFailure](err); ok {
		outcome = revocationOutcomes[failure.RevocationOutcome()]
	}
	return audit.VerificationFailed(code, err.Error(), outcome)
}
