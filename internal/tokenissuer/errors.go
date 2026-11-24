package tokenissuer

import (
	"errors"

	"github.com/tmknom/gates/internal/revocation"
)

// ErrVerificationFailure は、発行結果が発行許可かポリシー読み取り許可と一致しないことを表す。
// 文言に含める情報と伏せる値は、Lambda Design の「応答とエラー文言に出す情報」に従う。
var ErrVerificationFailure = errors.New("verification failure")

// VerificationFailure は、利用者に返すトークンの照合失敗のエラーで、ErrVerificationFailure を満たし、失効の結果を持つ。
// ポリシー読み取りトークンの照合失敗はこの型にせず、その失効の結果は取得の段階が別に返す。
type VerificationFailure struct {
	// verification は、ErrVerificationFailure を包んだ照合の失敗である
	verification error
	outcome      revocation.Outcome
	// revocationErr は、失効を試みて失敗したときの理由である。
	// 失効の失敗は失敗の種別ではないため Unwrap に含めず、照合失敗をほかの種別として判定させない
	revocationErr error
}

// Error は、照合の失敗の文言に、失効を試みて失敗した場合だけその理由を添える。
func (e *VerificationFailure) Error() string {
	if e.outcome == revocation.Failed {
		return e.verification.Error() + "; " + e.revocationErr.Error()
	}
	return e.verification.Error()
}

// Unwrap は、照合の失敗を返す。
func (e *VerificationFailure) Unwrap() error {
	return e.verification
}

// RevocationOutcome は、失効の結果を返す。
func (e *VerificationFailure) RevocationOutcome() revocation.Outcome {
	return e.outcome
}
